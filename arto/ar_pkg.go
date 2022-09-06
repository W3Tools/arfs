package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"

	"github.com/everFinance/goar"
	"github.com/everFinance/goar/types"
	"github.com/everFinance/goar/utils"
)

type ArUploader struct {
	node   string
	wallet *goar.Wallet
	client *goar.Client
}

type ArweaveCfg struct {
	Node    string `toml:"node,omitempty"`
	KeyFile string `toml:"key_file,omitempty"`
}

func InitArweave(keyFile string) (*ArUploader, error) {
	cfg := ArweaveCfg{
		Node:    "https://arweave.net/",
		KeyFile: keyFile,
	}

	return InitArweave2(&cfg)
}

func InitArweave2(cfg *ArweaveCfg) (*ArUploader, error) {
	wallet, err := goar.NewWalletFromPath(cfg.KeyFile, cfg.Node)
	if err != nil {
		return nil, err
	}

	ar := &ArUploader{
		node:   cfg.Node,
		wallet: wallet,
		client: wallet.Client,
	}
	return ar, nil
}

func (uploader *ArUploader) GetBalance() {
	info, err := uploader.client.GetInfo()
	if err != nil {
		fmt.Printf("err: %v\n", err)
		return
	}
	fmt.Printf("info: %v\n", info)
	// p, err := uploader.client.GetWalletBalance()
}

func (uploader *ArUploader) GetTxPrice(data []byte) (int64, error) {
	return uploader.client.GetTransactionPrice(data, nil)
}

func (uploader *ArUploader) Upload(data []byte, manifest bool) (*types.Transaction, error) {
	tx, err := uploader.GetTx(data, manifest)
	if err != nil {
		return nil, err
	}
	txUploader, err := goar.CreateUploader(uploader.client, tx, nil)
	if err != nil {
		return nil, fmt.Errorf("goar.CreateUploader err: %v", err)
	}
	err = txUploader.Once()
	if err != nil {
		return nil, fmt.Errorf("uploader.Once err: %v", err)
	}

	return tx, nil
}

func (uploader *ArUploader) UploadCallTxHash(data []byte, manifest bool) (txHash string, err error) {
	tx, err := uploader.Upload(data, manifest)
	if err != nil {
		return "", err
	}
	return tx.ID, err
}

func (uploader *ArUploader) UploadCallUrl(data []byte, manifest bool) (arLink string, err error) {
	tx, err := uploader.Upload(data, manifest)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(uploader.node)
	if err != nil {
		err = fmt.Errorf("parse failed: %w", err)
		return
	}
	u.Path = path.Join(u.Path, tx.ID)

	return u.String(), nil
}

func (uploader *ArUploader) GetTx(data []byte, manifest bool) (*types.Transaction, error) {
	anchor, err := uploader.client.GetTransactionAnchor()
	if err != nil {
		return nil, err
	}
	reward, err := uploader.GetTxPrice(data)
	if err != nil {
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	var tags []types.Tag
	if manifest {
		tags = append(tags, types.Tag{
			Name:  "Content-Type",
			Value: "application/x.arweave-manifest+json",
		})
	} else {
		tags = append(tags, types.Tag{
			Name:  "Content-Type",
			Value: http.DetectContentType(data),
		})
		tags = append(tags, types.Tag{
			Name:  "User-Agent",
			Value: "W3Tools",
		})
		tags = append(tags, types.Tag{
			Name:  "FileHash",
			Value: getFileHash(data),
		})
	}

	tx := &types.Transaction{
		Format:   2,
		Target:   "",
		Quantity: "0",
		Tags:     utils.TagsEncode(tags),
		Data:     utils.Base64Encode(data),
		DataSize: fmt.Sprintf("%d", len(data)),
		Reward:   fmt.Sprintf("%d", reward*(100)/100),
		LastTx:   anchor,
		Owner:    utils.Base64Encode(uploader.wallet.Signer.PubKey.N.Bytes()),
	}

	err = utils.SignTransaction(tx, uploader.wallet.Signer.PrvKey)
	if err != nil {
		return nil, err
	}
	return tx, nil
}

func getFileHash(data []byte) string {
	reader := bytes.NewReader(data)
	hash := sha256.New()
	if _, err := io.Copy(hash, reader); err != nil {
		fmt.Printf("[GetFileHash] io.Copy err, msg: %v\n", err)
		return ""
	}
	sum := hash.Sum(nil)
	return fmt.Sprintf("%x", sum)
}
