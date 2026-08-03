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
	file, err := os.Open("/tmp/manifest.yaml")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	
	// Create the chart directory if it doesn't exist
	chartDir := "/home/danilo.nicioka/git/lab/noc/argocd/helm/sistemas-administrativos/portal-de-servicos/chart"
	os.RemoveAll(chartDir)
	os.MkdirAll(chartDir, 0755)
	
	os.Chdir("/home/danilo.nicioka/git/lab/noc/argocd/helm/sistemas-administrativos/portal-de-servicos")
	
	conf := config.Config{ChartName: "chart"}
	objects := bufio.NewReader(file)
	trans := k8smanifest.New(conf, objects)
	
	engine := app.NewEngine(conf, helm.NewOutput())
	err = engine.Run(context.Background(), trans)
	if err != nil {
		panic(err)
	}
	fmt.Println("Chart generated successfully in", chartDir)
}
