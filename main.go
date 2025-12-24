package main

import (
	"maven_repo/logger"
	"maven_repo/server"

	"go.uber.org/fx"
)

func main() {
	fx.New(
		logger.Module,
		server.Module,
	).Run()
}
