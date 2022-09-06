package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

// 批量上传文件
const (
	ManifestFile        = "manifest.w3tools"
	ManifestContentType = "application/x.arweave-manifest+json"
	IndexFile           = "index.html"
)

type ManifestStruct struct {
	Manifest string                          `json:"manifest"`
	Version  string                          `json:"version"`
	Index    ManifestIndexStruct             `json:"index"`
	Paths    map[string]ManifestPathIdStruct `json:"paths"`
}

type ManifestIndexStruct struct {
	Path string `json:"path"`
}

type ManifestPathIdStruct struct {
	ID string `json:"id"`
}

func main() {
	dir := flag.String("d", "./tmp1", "部署目标")
	flag.Parse()

	// 对于部署整个文件夹, 先创建一个manifest. 统一rootHash
	manifestFile := fmt.Sprintf("%s/manifest.w3tools", *dir)
	manifest := GenerateManifest(manifestFile)
	if manifest == nil {
		return
	}

	// 初始化arweave客户端
	SecretPath := "../etc/ar.json"
	uploader, err := InitArweave(SecretPath)
	if err != nil {
		fmt.Printf("InitArweave err, msg: %v\n", err)
		return
	}

	// 文件处理
	rd, err := os.ReadDir(*dir)
	if err != nil {
		fmt.Printf("os.ReadDir err, msg: %v\n", err)
		return
	}

	for k, v := range rd {
		// 写index配置
		if k == 0 && manifest.Index.Path == "" {
			manifest.Index.Path = v.Name()
		}
		if v.Name() == IndexFile {
			manifest.Index.Path = v.Name()
		}
		// 如果是manifest文件，则跳过
		if v.Name() == ManifestFile {
			continue
		}

		if _, ok := manifest.Paths[v.Name()]; ok {
			fmt.Printf("id: %v\n", manifest.Paths[v.Name()].ID)
			continue
		}

		// 生成hash
		absPath := fmt.Sprintf("%s/%s", *dir, v.Name())

		data, err := os.ReadFile(absPath)
		if err != nil {
			fmt.Printf("os.ReadFile<%s> err, msg: %v", v.Name(), err)
			return
		}
		txHash, err := uploader.UploadCallTxHash(data, false)
		uploader.client.GetTransactionStatus(txHash)
		if err != nil {
			fmt.Printf("uploader.GetTx<%s> err, msg: %v", v.Name(), err)
			return
		}

		manifest.Paths[v.Name()] = ManifestPathIdStruct{
			ID: txHash,
		}

		// 写入manifest文件
		manifestData, err := json.Marshal(manifest)
		if err != nil {
			fmt.Printf("json.Marshal err: %s\n", err)
			return
		}

		err = WriteManifest(manifestFile, manifestData)
		if err != nil {
			return
		}
	}

	// 上传manifest文件
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		fmt.Printf("json.Marshal err: %s\n", err)
		return
	}
	fileHash, err := uploader.UploadCallUrl(manifestData, true)
	if err != nil {
		fmt.Printf("uploader.UploadCallTxHash err, msg: %v\n", err)
		return
	}
	fmt.Printf("rootHash: %v\n", fileHash)
}

func GenerateManifest(path string) *ManifestStruct {
	var manifest = &ManifestStruct{}
	_, err := os.Stat(path)

	if os.IsNotExist(err) {
		fmt.Printf("[GenerateManifest] Generate manifest: %s\n", path)
		manifest = NewManifest()

		data, err := json.Marshal(manifest)
		if err != nil {
			fmt.Printf("[GenerateManifest] json.Marshal err: %s\n", path)
			return nil
		}

		err = WriteManifest(path, data)
		if err != nil {
			return nil
		}
		return manifest
	}

	fmt.Printf("[GenerateManifest] read manifest: %s\n", path)
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("[GenerateManifest] os.ReadFile err, msg: %v\n", err)
		return nil
	}

	if manifest.Paths == nil {
		manifest.Paths = make(map[string]ManifestPathIdStruct)
	}

	err = json.Unmarshal(data, manifest)
	if err != nil {
		fmt.Printf("[GenerateManifest] json.Unmarshal err, msg: %v\n", err)
		return nil
	}

	return manifest
}

func NewManifest() *ManifestStruct {
	manifest := &ManifestStruct{
		Manifest: "arweave/paths",               // 固定格式
		Version:  "0.1.0",                       // 固定格式
		Index:    ManifestIndexStruct{Path: ""}, // 固定格式
		Paths:    make(map[string]ManifestPathIdStruct),
	}

	return manifest
}

func WriteManifest(name string, data []byte) error {
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("[WriteManifest] os.Open err, msg: %v\n", err)
		return err
	}

	defer f.Close()

	_, err = f.Write(data)
	if err != nil {
		fmt.Printf("[WriteManifest] f.Write err, msg: %v\n", err)
		return err
	}

	return nil
}
