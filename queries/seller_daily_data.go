package queries

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func GetSellerDataQuery(sellerID string) bson.A {
	return bson.A{
		bson.D{
			{Key: "$match",
				Value: bson.D{
					{Key: "seller_id", Value: sellerID},
					{Key: "date", Value: bson.D{{Key: "$gte", Value: time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)}}},
				},
			},
		},
		bson.D{
			{Key: "$group",
				Value: bson.D{
					{Key: "_id", Value: primitive.Null{}},
					{Key: "revenue", Value: bson.D{{Key: "$sum", Value: "$revenue"}}},
				},
			},
		},
		bson.D{
			{Key: "$project",
				Value: bson.D{
					{Key: "_id", Value: 0},
					{Key: "revenue", Value: 1},
				},
			},
		},
	}
}
