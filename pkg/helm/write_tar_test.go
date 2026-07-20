package helm_test

import (
    "archive/tar"
    "compress/gzip"
    "io"
    "os"
    "testing"

    "github.com/arttor/helmify/pkg/helm"
)

func TestWriteTarGzLocal(t *testing.T) {
    files := map[string][]byte{
        "templates/foo.yaml": []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n"),
        "Chart.yaml":        []byte("apiVersion: v2\nname: testchart\nversion: 0.1.0\n"),
    }
    outPath := "/tmp/out_test.tar.gz"
    f, err := os.Create(outPath)
    if err != nil {
        t.Fatalf("create file: %v", err)
    }
    defer f.Close()
    if err := helm.WriteTarGz(files, "testchart", f); err != nil {
        t.Fatalf("WriteTarGz: %v", err)
    }
    // Re-open and read the tar to ensure no extraction errors
    f2, err := os.Open(outPath)
    if err != nil {
        t.Fatalf("open tar: %v", err)
    }
    defer f2.Close()
    gz, err := gzip.NewReader(f2)
    if err != nil {
        t.Fatalf("gzip reader: %v", err)
    }
    tr := tar.NewReader(gz)
    for {
        _, err := tr.Next()
        if err == io.EOF {
            break
        }
        if err != nil {
            t.Fatalf("tar read error: %v", err)
        }
    }
}
