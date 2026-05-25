package service

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const (
	selEmail    = `input[type="email"], input[name="email"], input[placeholder*="mail" i]`
	selPassword = `input[type="password"], input[name="password"]`
	selSubmit   = `button[type="submit"]`

	pageRenderWait = 6 * time.Second
)

// crawlRoutes are the dashboard pages inspected per seller. name is used in findings.
// isProfit marks profit-dashboard tabs, which collapse into one finding when all fail.
var crawlRoutes = []struct {
	name, path string
	isProfit   bool
}{
	{"home", "/seller/dashboard", false},
	{"overview", "/seller/profit-dashboard/overview", true},
	{"revenue", "/seller/profit-dashboard/revenue", true},
	{"profits", "/seller/profit-dashboard/seller-profits", true},
	{"inventory", "/seller/profit-dashboard/seller-inventory", true},
	{"regional_inventory", "/seller/profit-dashboard/regional-inventory", true},
	{"refunds", "/seller/profit-dashboard/refunds", true},
	{"traffic", "/seller/profit-dashboard/traffic", true},
	{"catalog", "/seller/profit-dashboard/catalog", true},
	{"ads", "/seller/ppc", false},
}

// jsMetric extracts a named metric card value from the profit overview page.
func jsMetric(label string) string {
	return fmt.Sprintf(`(function(){
		const cards = document.querySelectorAll('app-metric-card, mat-card, [class*="card"]');
		for (const card of cards) {
			if (new RegExp(%q, 'i').test(card.innerText)) {
				const val = card.querySelector('span[class*="text-xl"], span[class*="font-semibold"]');
				if (val) return val.innerText.trim();
			}
		}
		return '';
	})()`, "^\\s*"+label)
}

// jsNoData counts visible "no data" placeholders, ignoring style/script noise.
const jsNoData = `[...document.querySelectorAll('div,p,span,h1,h2,h3,h4')].filter(el =>
	el.children.length === 0 && el.offsetParent !== null &&
	/^(no data|no data available|no records found|no records|nothing to show|data not available|no result)/i.test((el.innerText||'').trim())
).length`

type apiFailure struct {
	status  int
	feature string
}

type pageResult struct {
	name        string
	unavailable bool
	timedOut    bool
	noData      bool
	jsException bool
}

type DashboardVerifier struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu      sync.Mutex
	apiFail []apiFailure
	jsErr   []string
}

func NewDashboardVerifier() (*DashboardVerifier, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.DisableGPU,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
	)
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	dv := &DashboardVerifier{ctx: ctx, cancel: cancel}

	// Capture failed API calls and uncaught JS exceptions throughout the session.
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventResponseReceived:
			if e.Response.Status >= 400 && strings.Contains(e.Response.URL, "api.sellerapp.com") {
				dv.mu.Lock()
				dv.apiFail = append(dv.apiFail, apiFailure{int(e.Response.Status), apiFeature(e.Response.URL)})
				dv.mu.Unlock()
			}
		case *runtime.EventExceptionThrown:
			dv.mu.Lock()
			dv.jsErr = append(dv.jsErr, e.ExceptionDetails.Text)
			dv.mu.Unlock()
		}
	})

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to launch browser: %w", err)
	}
	return dv, nil
}

func (dv *DashboardVerifier) Close() { dv.cancel() }

// drain returns and clears the errors captured since the last drain.
func (dv *DashboardVerifier) drain() ([]apiFailure, []string) {
	dv.mu.Lock()
	defer dv.mu.Unlock()
	af, je := dv.apiFail, dv.jsErr
	dv.apiFail, dv.jsErr = nil, nil
	return af, je
}

// switchToSeller points the browser at the given seller, logging in if needed.
func (dv *DashboardVerifier) switchToSeller(accountId string) error {
	baseURL := os.Getenv("DASHBOARD_BASE_URL")
	email := os.Getenv("DASHBOARD_ADMIN_EMAIL")
	password := os.Getenv("DASHBOARD_ADMIN_PASSWORD")

	ctx, cancel := context.WithTimeout(dv.ctx, 45*time.Second)
	defer cancel()

	loginURL := fmt.Sprintf("%s/admin-login?user_id=%s", baseURL, accountId)
	var currentURL string

	return chromedp.Run(ctx,
		chromedp.Navigate(loginURL),
		chromedp.Sleep(2*time.Second),
		chromedp.ActionFunc(func(ctx context.Context) error {
			if err := chromedp.Location(&currentURL).Do(ctx); err != nil {
				return err
			}
			if !strings.Contains(currentURL, "admin-login") {
				return nil // already authenticated
			}
			return chromedp.Run(ctx,
				chromedp.WaitVisible(selEmail, chromedp.ByQuery),
				chromedp.Click(selEmail, chromedp.ByQuery),
				chromedp.SendKeys(selEmail, email, chromedp.ByQuery),
				chromedp.Click(selPassword, chromedp.ByQuery),
				chromedp.SendKeys(selPassword, password, chromedp.ByQuery),
				chromedp.WaitEnabled(selSubmit, chromedp.ByQuery),
				chromedp.Click(selSubmit, chromedp.ByQuery),
				chromedp.ActionFunc(func(ctx context.Context) error {
					deadline := time.Now().Add(20 * time.Second)
					for time.Now().Before(deadline) {
						if err := chromedp.Location(&currentURL).Do(ctx); err != nil {
							return err
						}
						if !strings.Contains(currentURL, "admin-login") {
							return nil
						}
						time.Sleep(500 * time.Millisecond)
					}
					return fmt.Errorf("still on login page after 20s")
				}),
			)
		}),
	)
}

// crawlPage navigates to one route and reports its state plus any API failures.
func (dv *DashboardVerifier) crawlPage(name, path string) (pageResult, []apiFailure) {
	baseURL := os.Getenv("DASHBOARD_BASE_URL")
	dv.drain() // discard errors from the previous page

	ctx, cancel := context.WithTimeout(dv.ctx, 30*time.Second)
	defer cancel()

	var landedURL string
	var noDataCount int
	err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL+path),
		chromedp.WaitReady("body"),
		chromedp.Sleep(pageRenderWait),
		chromedp.Location(&landedURL),
		chromedp.Evaluate(jsNoData, &noDataCount),
	)
	res := pageResult{name: name}
	if err != nil {
		res.timedOut = true
		return res, nil
	}

	apiFail, jsErr := dv.drain()

	// A route that bounces back to /seller/dashboard is unavailable for this seller.
	if path != "/seller/dashboard" && strings.Contains(landedURL, "/seller/dashboard") {
		res.unavailable = true
		return res, apiFail
	}

	res.noData = noDataCount > 0
	res.jsException = len(jsErr) > 0
	return res, apiFail
}

// GetFindings logs in as the seller, crawls every dashboard page, and returns
// a pipe-separated finding string for the CSV.
func (dv *DashboardVerifier) GetFindings(accountId string) (string, error) {
	if accountId == "" {
		return "no_account_id", nil
	}

	if err := dv.switchToSeller(accountId); err != nil {
		return "login_failed:" + sanitize(err.Error()), nil
	}

	var results []pageResult
	var allAPIFail []apiFailure
	for _, r := range crawlRoutes {
		res, af := dv.crawlPage(r.name, r.path)
		results = append(results, res)
		allAPIFail = append(allAPIFail, af...)
	}

	findings := buildPageFindings(results)

	// Deduped API errors — the same failing endpoint on many pages is one finding.
	findings = append(findings, dedupAPIFailures(allAPIFail)...)

	// UI-only metric checks (no DB comparison).
	findings = append(findings, dv.metricFindings()...)

	return strings.Join(findings, "|"), nil
}

// buildPageFindings turns per-page results into findings, collapsing the case
// where every profit-dashboard tab is unavailable into a single finding.
func buildPageFindings(results []pageResult) []string {
	allProfitDown := allProfitTabsDown(results)

	var findings []string
	if allProfitDown {
		findings = append(findings, "profit_dashboard_unavailable")
	}
	for i, res := range results {
		findings = append(findings, pageFindings(res, crawlRoutes[i].isProfit, allProfitDown)...)
	}
	return findings
}

// allProfitTabsDown reports whether every profit-dashboard tab is unavailable.
func allProfitTabsDown(results []pageResult) bool {
	total, down := 0, 0
	for i, r := range crawlRoutes {
		if r.isProfit {
			total++
			if results[i].unavailable {
				down++
			}
		}
	}
	return total > 0 && down == total
}

// pageFindings returns the findings for a single crawled page.
func pageFindings(res pageResult, isProfit, allProfitDown bool) []string {
	if res.timedOut {
		return []string{res.name + "_page_timeout"}
	}
	var f []string
	if res.unavailable && !(isProfit && allProfitDown) {
		f = append(f, res.name+"_section_unavailable")
	}
	if res.noData {
		f = append(f, res.name+"_no_data")
	}
	if res.jsException {
		f = append(f, res.name+"_js_exception")
	}
	return f
}

// metricFindings inspects the overview page's metric cards for UI-only anomalies.
func (dv *DashboardVerifier) metricFindings() []string {
	baseURL := os.Getenv("DASHBOARD_BASE_URL")
	ctx, cancel := context.WithTimeout(dv.ctx, 25*time.Second)
	defer cancel()

	var netProfitStr string
	err := chromedp.Run(ctx,
		chromedp.Navigate(baseURL+"/seller/profit-dashboard/overview"),
		chromedp.WaitReady("body"),
		chromedp.Sleep(pageRenderWait),
		chromedp.Evaluate(jsMetric("Net Profit"), &netProfitStr),
	)
	if err != nil {
		return nil
	}

	var findings []string
	if netProfit, err := parseRevenue(netProfitStr); err == nil && netProfit < 0 {
		findings = append(findings, "net_profit_negative")
	}
	return findings
}

// apiFeature reduces an API URL like /ams/amazon/in/profile/actions/history
// to a short "profile_actions" feature label.
func apiFeature(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "api"
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if p == "amazon" && i+2 < len(parts) {
			feat := parts[i+2:]
			if len(feat) > 2 {
				feat = feat[:2]
			}
			return strings.Join(feat, "_")
		}
	}
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "api"
}

// dedupAPIFailures collapses repeated endpoint failures into one finding each.
func dedupAPIFailures(fails []apiFailure) []string {
	seen := map[string]bool{}
	var out []string
	for _, f := range fails {
		finding := fmt.Sprintf("%s_api_error:%d", f.feature, f.status)
		if !seen[finding] {
			seen[finding] = true
			out = append(out, finding)
		}
	}
	sort.Strings(out)
	return out
}

func parseRevenue(s string) (float64, error) {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer(",", "", "$", "", "₹", "", "£", "", "€", "").Replace(s)
	multiplier := 1.0
	lower := strings.ToLower(s)
	if strings.HasSuffix(lower, "m") {
		multiplier = 1_000_000
		s = s[:len(s)-1]
	} else if strings.HasSuffix(lower, "k") {
		multiplier = 1_000
		s = s[:len(s)-1]
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return val * multiplier, nil
}

func sanitize(s string) string {
	return strings.ReplaceAll(strings.TrimSpace(s), "|", ";")
}
