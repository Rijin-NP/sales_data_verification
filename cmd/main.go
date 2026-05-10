package main

import (
	db "dataverification/client"
	dao "dataverification/dao"
	handlers "dataverification/handlers"
	"dataverification/router"
	"fmt"
	"net/http"
)

func main() {
	mongoClient, ctx, err := db.Connect("mongodb://root:TEf0mU28XWN6mUH@127.0.0.1:27017/")
	if err != nil {
		panic(err)
	}
	defer mongoClient.Disconnect(ctx)

	service := dao.MongoDBInstance{
		Client: mongoClient,
		Ctx:    ctx,
	}

	handler := handlers.NewHandlers(service)
	r := router.NewRouter(handler)
	fmt.Println("Starting at 9070")
	http.ListenAndServe(":9070", r)

}
