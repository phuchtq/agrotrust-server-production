package main

import (
	_ "raise-child/business"
	"raise-child/cmd"
)

// @title AgroTrust Server API
// @version 1.0
// @description API for AgroTrust Server
// @host agrotrust-server-production.onrender.com
// @BasePath /
// @schemes https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	cmd.Execute()
}
