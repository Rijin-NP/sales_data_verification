package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

const sellerID = "JPaqNkcAlhVi85qSbS5SomjIiu92" // tools@kamecommerce.in (has data)

func main() {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.DisableGPU,
	)
	allocCtx, _ := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	ctx, cancel2 := context.WithTimeout(ctx, 180*time.Second)
	defer cancel2()

	// capture console errors + failed network
	var mu sync.Mutex
	var consoleErrs, netErrs []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		mu.Lock()
		defer mu.Unlock()
		switch e := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if e.Type == "error" {
				parts := []string{}
				for _, a := range e.Args {
					parts = append(parts, fmt.Sprintf("%s", a.Value))
				}
				consoleErrs = append(consoleErrs, strings.Join(parts, " "))
			}
		case *runtime.EventExceptionThrown:
			consoleErrs = append(consoleErrs, "EXCEPTION: "+e.ExceptionDetails.Text)
		case *network.EventResponseReceived:
			if e.Response.Status >= 400 {
				netErrs = append(netErrs, fmt.Sprintf("%d %s", e.Response.Status, e.Response.URL))
			}
		case *network.EventLoadingFailed:
			netErrs = append(netErrs, "FAILED "+e.ErrorText)
		}
	})

	// login
	loggedIn := false
	err := chromedp.Run(ctx,
		network.Enable(),
		chromedp.Navigate("https://dashboard.sellerapp.com/admin-login?user_id="+sellerID),
		chromedp.WaitVisible(`input[type="email"]`, chromedp.ByQuery),
		chromedp.Click(`input[type="email"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[type="email"]`, "brij@sellerapp.com", chromedp.ByQuery),
		chromedp.Click(`input[type="password"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[type="password"]`, "SellerApp@1234", chromedp.ByQuery),
		chromedp.WaitEnabled(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.ActionFunc(func(ctx context.Context) error {
			deadline := time.Now().Add(20 * time.Second)
			for time.Now().Before(deadline) {
				var u string
				chromedp.Location(&u).Do(ctx)
				if !strings.Contains(u, "admin-login") {
					loggedIn = true
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
			return nil
		}),
	)
	if err != nil || !loggedIn {
		fmt.Println("login failed:", err)
		os.Exit(1)
	}
	fmt.Println("LOGIN OK")

	// dump nav routes
	var navRoutes string
	chromedp.Run(ctx,
		chromedp.Sleep(3*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('a[href]')].map(a=>a.getAttribute('href')).filter(h=>h&&h.startsWith('/seller')).filter((v,i,a)=>a.indexOf(v)===i).join('\n')`, &navRoutes),
	)
	fmt.Println("=== NAV ROUTES ===")
	fmt.Println(navRoutes)

	// visit profit-dashboard and dump tab routes
	var tabRoutes string
	chromedp.Run(ctx,
		chromedp.Navigate("https://dashboard.sellerapp.com/seller/profit-dashboard/overview"),
		chromedp.Sleep(5*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('a[href]')].map(a=>a.getAttribute('href')).filter(h=>h&&h.includes('profit-dashboard')).filter((v,i,a)=>a.indexOf(v)===i).join('\n')`, &tabRoutes),
	)
	fmt.Println("=== PROFIT-DASHBOARD TABS ===")
	fmt.Println(tabRoutes)

	// crawl each candidate route, capture errors + empty-state text
	routes := []string{
		"/seller/dashboard",
		"/seller/profit-dashboard/overview",
		"/seller/profit-dashboard/revenue",
		"/seller/profit-dashboard/profits",
		"/seller/profit-dashboard/inventory",
		"/seller/profit-dashboard/refunds",
		"/seller/profit-dashboard/traffic",
	}
	for _, r := range routes {
		mu.Lock()
		consoleErrs = nil
		netErrs = nil
		mu.Unlock()

		var pageURL, emptyStates string
		chromedp.Run(ctx,
			chromedp.Navigate("https://dashboard.sellerapp.com"+r),
			chromedp.Sleep(6*time.Second),
			chromedp.Location(&pageURL),
			chromedp.Evaluate(`[...document.querySelectorAll('*')].filter(el=>el.children.length===0&&/no data|not found|no records|nothing|empty|coming soon/i.test(el.innerText||'')).map(el=>el.innerText.trim()).filter((v,i,a)=>a.indexOf(v)===i).slice(0,5).join(' || ')`, &emptyStates),
		)
		mu.Lock()
		ce := append([]string{}, consoleErrs...)
		ne := append([]string{}, netErrs...)
		mu.Unlock()

		fmt.Printf("\n--- ROUTE %s ---\n", r)
		fmt.Println("  landed:", pageURL)
		fmt.Println("  empty-states:", emptyStates)
		fmt.Printf("  console errors (%d):\n", len(ce))
		for _, c := range ce {
			if len(c) > 160 {
				c = c[:160]
			}
			fmt.Println("    -", c)
		}
		fmt.Printf("  network 4xx/5xx (%d):\n", len(ne))
		seen := map[string]bool{}
		for _, n := range ne {
			if seen[n] {
				continue
			}
			seen[n] = true
			fmt.Println("    -", n)
		}
	}
}
