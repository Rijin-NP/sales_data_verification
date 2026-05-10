package queries

import "go.mongodb.org/mongo-driver/bson"

func GetProductCountQuery(profileID string) bson.A {
	return bson.A{
		bson.D{
			{Key: "$match",
				Value: bson.D{
					{Key: "profile_id", Value: profileID},
					{Key: "sync_type", Value: "INITIAL_SYNC"},
					{Key: "report_options.report_type", Value: "GET_FLAT_FILE_ALL_ORDERS_DATA_BY_ORDER_DATE_GENERAL"},
				},
			},
		},
		bson.D{
			{Key: "$group",
				Value: bson.D{
					{Key: "_id", Value: "$profile_id"},
					{Key: "product_count", Value: bson.D{{Key: "$first", Value: "$product_count"}}},
				},
			},
		},
	}
}
