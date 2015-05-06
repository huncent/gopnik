package tileserver

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"

	"git.apache.org/thrift.git/lib/go/thrift"
	json "github.com/orofarne/strict-json"
	"github.com/stretchr/testify/require"

	"app"
	"gopnik"
	"gopnikrpc"
	"gopnikrpc/types"
	"plugins"
	_ "plugins_enabled"
	"sampledata"
)

type Config struct {
	CachePlugin string                     //
	Plugins     map[string]json.RawMessage //
}

func TestSimple(t *testing.T) {
	addr := "127.0.0.1:5341"

	cfg := []byte(`{
		"UseMultilevel": true,
		"Backend": {
			"Plugin":       "MemoryKV",
			"PluginConfig": {}
		}
	}`)
	renderPoolsConfig := app.RenderPoolsConfig{
		[]app.RenderPoolConfig{
			app.RenderPoolConfig{
				Cmd:       sampledata.SlaveCmd, // Render slave binary
				MinZoom:   0,
				MaxZoom:   19,
				PoolSize:  1,
				QueueSize: 10,
				RenderTTL: 0,
			},
		},
	}

	cpI, err := plugins.DefaultPluginStore.Create("KVStorePlugin", json.RawMessage(cfg))
	if err != nil {
		log.Fatal(err)
	}
	cp, ok := cpI.(gopnik.CachePluginInterface)
	if !ok {
		log.Fatal("Invalid cache plugin type")
	}

	ts, err := NewTileServer(renderPoolsConfig, cp, time.Duration(0))
	if err != nil {
		log.Fatalf("Failed to create tile server: %v", err)
	}

	s := &http.Server{
		Addr:           addr,
		Handler:        ts,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go s.ListenAndServe()

	resp, err := http.Get(fmt.Sprintf("http://%s/1/0/0.png", addr))
	if err != nil {
		t.Errorf("http.Get error: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("ioutil.ReadAll: %v", err)
	}

	require.NotNil(t, body)

	sampledata.CheckTile(t, body, "1_0_0.png")
}

func TestThriftSimple(t *testing.T) {
	addr := "127.0.0.1:5342"

	cfg := []byte(`{
		"UseMultilevel": true,
		"Backend": {
			"Plugin":       "MemoryKV",
			"PluginConfig": {}
		}
	}`)
	renderPoolsConfig := app.RenderPoolsConfig{
		[]app.RenderPoolConfig{
			app.RenderPoolConfig{
				Cmd:       sampledata.SlaveCmd, // Render slave binary
				MinZoom:   0,
				MaxZoom:   19,
				PoolSize:  1,
				QueueSize: 10,
				RenderTTL: 0,
			},
		},
	}

	cpI, err := plugins.DefaultPluginStore.Create("KVStorePlugin", json.RawMessage(cfg))
	if err != nil {
		t.Fatal(err)
	}
	cp, ok := cpI.(gopnik.CachePluginInterface)
	if !ok {
		t.Fatal("Invalid cache plugin type")
	}

	ts, err := NewTileServer(renderPoolsConfig, cp, time.Duration(0))
	if err != nil {
		t.Fatalf("Failed to create tile server: %v", err)
	}
	go func() {
		err := RunServer(addr, ts)
		if err != nil {
			panic(err)
		}
	}()
	time.Sleep(time.Millisecond)

	transportFactory := thrift.NewTFramedTransportFactory(thrift.NewTTransportFactory())
	protocolFactory := thrift.NewTBinaryProtocolFactoryDefault()
	socket, err := thrift.NewTSocket(addr)
	require.Nil(t, err)
	transport := transportFactory.GetTransport(socket)
	defer transport.Close()
	err = transport.Open()
	if err != nil {
		t.Errorf("transport open: %v", err.Error())
	}

	renderClient := gopnikrpc.NewRenderClientFactory(transport, protocolFactory)
	tile, err := renderClient.Render(&types.Coord{
		Zoom: 1,
		X:    0,
		Y:    0,
		Size: 1,
	},
		gopnikrpc.Priority_HIGH, false)
	require.Nil(t, err)
	require.NotNil(t, tile)
	require.NotNil(t, tile.Image)

	sampledata.CheckTile(t, tile.Image, "1_0_0.png")
}
