package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"time"

	"cloud.google.com/go/storage"
	"github.com/incognitochain/incognito-chain/common"
	"google.golang.org/api/option"
)

// CheckForceUpdateSourceCode - loop to check current version with update version is equal
// Force source code to be updated and remove data
func (serverObject Server) CheckForceUpdateSourceCode() {
	go func() {
		ctx := context.Background()
		myClient, err := storage.NewClient(ctx, option.WithoutAuthentication())
		if err != nil {
			Logger.log.Error(err)
		}
		for {
			reader, err := myClient.Bucket("incognito").Object(serverObject.chainParams.ChainVersion).NewReader(ctx)
			if err != nil {
				Logger.log.Error(err)
				time.Sleep(10 * time.Second)
				continue
			}
			defer reader.Close()

			type VersionChain struct {
				Version    string `json:"Version"`
				Note       string `json:"Note"`
				RemoveData bool   `json:"RemoveData"`
			}
			versionChain := VersionChain{}
			currentVersion := version()
			body, err := ioutil.ReadAll(reader)
			if err != nil {
				Logger.log.Error(err)
				time.Sleep(10 * time.Second)
				continue
			}
			err = json.Unmarshal(body, &versionChain)
			if err != nil {
				Logger.log.Error(err)
				time.Sleep(10 * time.Second)
				continue
			}
			force := currentVersion != versionChain.Version
			if force {
				Logger.log.Error("\n*********************************************************************************\n" +
					versionChain.Note +
					"\n*********************************************************************************\n")
				Logger.log.Error("\n*********************************************************************************\n You're running version: " +
					currentVersion +
					"\n*********************************************************************************\n")
				Logger.log.Error("\n*********************************************************************************\n" +
					versionChain.Note +
					"\n*********************************************************************************\n")

				Logger.log.Error("\n*********************************************************************************\n New version: " +
					versionChain.Version +
					"\n*********************************************************************************\n")

				Logger.log.Error("\n*********************************************************************************\n" +
					"We're exited because having a force update on this souce code." +
					"\nPlease Update source code at https://github.com/incognitochain/incognito-chain" +
					"\n*********************************************************************************\n")
				if versionChain.RemoveData {
					serverObject.Stop()
					errRemove := os.RemoveAll(cfg.DataDir)
					if errRemove != nil {
						Logger.log.Error("We NEEDD to REMOVE database directory but can not process by error", errRemove)
					}
					time.Sleep(60 * time.Second)
				}
				os.Exit(common.ExitCodeForceUpdate)
			}
			time.Sleep(10 * time.Second)
		}
	}()
}
