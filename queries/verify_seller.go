package queries

import (
	"time"

	"go.mongodb.org/mongo-driver/bson"
)

func GetSellerVerificationQuery(startDate, endDate time.Time) bson.A {
	return bson.A{
		bson.D{
			{Key: "$match",
				Value: bson.D{
					{Key: "created_at",
						Value: bson.D{
							{Key: "$gte", Value: startDate},
							{Key: "$lt", Value: endDate},
						},
					},
				},
			},
		},
		bson.D{
			{Key: "$project",
				Value: bson.D{
					{Key: "_id", Value: 0},
					{Key: "profile_id", Value: 1},
					{Key: "account_id", Value: 1},
					{Key: "geo", Value: 1},
					{Key: "created_at", Value: 1},
					{Key: "email", Value: 1},
					{Key: "last_analyzed_at", Value: 1},
					{Key: "refresh_token", Value: 1},
				},
			},
		},
	}
}
