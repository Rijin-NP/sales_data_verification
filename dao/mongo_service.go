package service

import (
	"bufio"
	"context"
	"os"
	"strconv"
	"time"

	"dataverification/models"
	"dataverification/queries"
	"encoding/csv"

	"go.mongodb.org/mongo-driver/mongo"
)

type MongoDBInstance struct {
	Client *mongo.Client
	Ctx    context.Context
}

func (md MongoDBInstance) Collection(dbName, collName string) *mongo.Collection {
	return md.Client.Database(dbName).Collection(collName)
}
func (md MongoDBInstance) VerifySellers(startDate, endDate time.Time) error {
	query := queries.GetSellerVerificationQuery(startDate, endDate)
	profileCursor, err := md.Collection("seller_auth_db", "profiles").Aggregate(context.Background(), query)
	if err != nil {
		return err
	}
	defer profileCursor.Close(md.Ctx)
	verifiedSellers := []*models.Profiles{}
	profileCursor.All(md.Ctx, &verifiedSellers)
	err = md.UpdateCsv(verifiedSellers)
	if err != nil {
		return err
	}
	return err

}

func (md MongoDBInstance) UpdateCsv(verifiedSellers []*models.Profiles) error {
	// update csv file
	file, err := os.Open("/Users/spurge/Downloads/verification_file.csv")
	if err != nil {
		return err
	}
	defer file.Close()
	reader := csv.NewReader(bufio.NewReader(file))
	existingRecords, err := reader.ReadAll()
	if err != nil {
		return err
	}

	existingData := make(map[string][]string)
	headers := existingRecords[0]
	for _, row := range existingRecords[1:] {
		key := row[0] + row[3] + row[2]
		existingData[key] = row
	}

	for _, data := range verifiedSellers {
		revenue, err := md.GetRevenueData(data.Geo, data.ProfileId)
		if err != nil {
			return err
		}
		productCount, err := md.GetProductCount(data.Geo, data.ProfileId)
		if err != nil {
			return err
		}
		createdAtStr := data.CreatedAt.Format("02-Jan-2006")
		key := data.ProfileId + data.Geo + data.Email
		var status, findings, products string
		products = strconv.FormatInt(productCount, 10)
		if data.LastAnalyzedAt == nil {
			status = "Syncing"
		} else {
			status = "Synced"
		}
		if existingRow, found := existingData[key]; found {
			existingRow[4] = status
			existingRow[5] = findings
			existingRow[6] = products
			existingData[key] = existingRow
		} else {
			existingData[key] = []string{
				data.ProfileId,
				createdAtStr,
				data.Email,
				data.Geo,
				status,
				findings,
				products,
				strconv.FormatFloat(revenue, 'f', 2, 64),
			}
		}

	}
	file, err = os.OpenFile("/Users/spurge/Downloads/verification_file.csv", os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(bufio.NewWriter(file))
	defer writer.Flush()
	err = writer.Write(headers)
	if err != nil {
		return err
	}
	for _, row := range existingData {
		err = writer.Write(row)
		if err != nil {
			return err
		}
	}
	return nil
}

func (md MongoDBInstance) GetRevenueData(geo, profileID string) (revenue float64, err error) {
	sellerCollection := "seller_daily_" + geo
	selleQuery := queries.GetSellerDataQuery(profileID)
	sellerDailyCursor, err := md.Collection("seller_dashboard_db", sellerCollection).Aggregate(context.Background(), selleQuery)
	if err != nil {
		return 0.00, nil
	}
	defer sellerDailyCursor.Close(md.Ctx)
	var sellerDataList []models.SellerDaily
	if err := sellerDailyCursor.All(md.Ctx, &sellerDataList); err != nil {
		return 0.00, err
	}
	if len(sellerDataList) > 0 {
		revenue = sellerDataList[0].Revenue
	} else {
		revenue = 0.00
	}

	return revenue, nil
}

func (md MongoDBInstance) GetProductCount(geo, profileID string) (productCount int64, err error) {
	ingestionCollection := "seller_ingestion_pool_" + geo
	productCountQuery := queries.GetProductCountQuery(profileID)
	productCountCursor, err := md.Collection("seller_ingestion_db", ingestionCollection).Aggregate(context.Background(), productCountQuery)
	if err != nil {
		return 0, err
	}
	defer productCountCursor.Close(md.Ctx)
	var result models.ProductCount
	if productCountCursor.Next(md.Ctx) {
		if err := productCountCursor.Decode(&result); err != nil {
			return 0, err
		}
		return result.Count, nil
	}
	return 0, nil
}
