package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/arttor/helmify/pkg/app"
	"github.com/arttor/helmify/pkg/config"
	"github.com/arttor/helmify/pkg/helm"
	"github.com/arttor/helmify/pkg/translator/k8smanifest"
)

func main() {
	file, err := os.Open("../test_data/sample-app.yaml")
	if err != nil {
		panic(err)
	}
	conf := config.Config{ChartName: "test-app"}
	objects := bufio.NewReader(file)
	trans := k8smanifest.New(conf, objects)
	engine := app.NewEngine(conf, helm.NewOutput())
	err = engine.Run(context.Background(), trans)
	if err != nil {
		panic(err)
	}
	fmt.Println("Done")
}
