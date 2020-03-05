package btc

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type BlockCypherClient struct {
}

// type of timestamp in blockheader is int64
// const API_KEY = "a2f2bad22feb460482efe5fbbefde77f"
var (
	blockTimestamp int64
	blockHeight    int
)

const ()

func (blockCypherClient *BlockCypherClient) GetNonceByTimestamp(startTime time.Time, maxTime time.Duration, timestamp int64) (int, int64, int64, error) {
	// Generated by curl-to-Go: https://mholt.github.io/curl-to-go
	resp, err := http.Get("https://api.blockcypher.com/v1/btc/main")
	if err != nil {
		return 0, 0, -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		chainBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return 0, 0, -1, NewBTCAPIError(UnExpectedError, err)
		}
		chain := make(map[string]interface{})
		err = json.Unmarshal(chainBytes, &chain)
		if err != nil {
			return 0, 0, -1, NewBTCAPIError(UnmashallJsonBlockError, err)
		}
		chainHeightFloat, ok := chain["height"].(float64)
		if !ok {
			return 0, 0, -1, NewBTCAPIError(WrongTypeError, errors.New("Height's Type should be float64"))
		}
		chainHeight := int(chainHeightFloat)
		chainTimestampString, ok := chain["time"].(string)
		if !ok {
			return 0, 0, -1, NewBTCAPIError(WrongTypeError, errors.New("Time's Type should be string"))
		}
		chainTimestamp, err := makeTimestamp2(chainTimestampString)
		if err != nil {
			return 0, 0, -1, NewBTCAPIError(TimeParseError, err)
		}
		blockHeight, err := estimateBlockHeight(blockCypherClient, timestamp, chainHeight, chainTimestamp, startTime, maxTime)
		if err != nil {
			return 0, 0, -1, err
		}
		blockTimestamp, _, err = blockCypherClient.GetTimeStampAndNonceByBlockHeight(blockHeight)
		if err != nil {
			return 0, 0, -1, err
		}
		if blockTimestamp == MaxTimeStamp {
			return 0, 0, -1, NewBTCAPIError(APIError, errors.New("Can't get result from API"))
		}
		if blockTimestamp > timestamp {
			for blockTimestamp > timestamp {
				blockHeight--
				blockTimestamp, _, err = blockCypherClient.GetTimeStampAndNonceByBlockHeight(blockHeight)
				if err != nil {
					return 0, 0, -1, err
				}
				if blockTimestamp == MaxTimeStamp {
					return 0, 0, -1, NewBTCAPIError(APIError, errors.New("Can't get result from API"))
				}
				if blockTimestamp <= timestamp {
					blockHeight++
					break
				}
			}
		} else {
			for blockTimestamp <= timestamp {
				blockHeight++
				if blockHeight > chainHeight {
					return 0, 0, -1, NewBTCAPIError(APIError, errors.New("Timestamp is greater than timestamp of highest block"))
				}
				blockTimestamp, _, err = blockCypherClient.GetTimeStampAndNonceByBlockHeight(blockHeight)
				if err != nil {
					return 0, 0, -1, err
				}
				if blockTimestamp == MaxTimeStamp {
					return 0, 0, -1, NewBTCAPIError(APIError, errors.New("Can't get result from API"))
				}
				if blockTimestamp > timestamp {
					break
				}
			}
		}
		timestamp, nonce, err := blockCypherClient.GetTimeStampAndNonceByBlockHeight(blockHeight)
		if err != nil {
			return 0, 0, -1, err
		}
		return blockHeight, timestamp, nonce, nil
	}
	return 0, 0, -1, NewBTCAPIError(NonceError, errors.New("Can't get nonce"))
}

func (blockCypherClient *BlockCypherClient) VerifyNonceWithTimestamp(startTime time.Time, maxTime time.Duration, timestamp int64, nonce int64) (bool, error) {
	_, _, tempNonce, err := blockCypherClient.GetNonceByTimestamp(startTime, maxTime, timestamp)
	if err != nil {
		return false, err
	}
	return tempNonce == nonce, nil
}

func (blockCypherClient *BlockCypherClient) GetCurrentChainTimeStamp() (int64, error) {
	resp, err := http.Get("https://api.blockcypher.com/v1/btc/main")
	if err != nil {
		return -1, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		chainBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return -1, err
		}
		chain := make(map[string]interface{})
		json.Unmarshal(chainBytes, &chain)
		chainTimestamp, err := makeTimestamp2(chain["time"].(string))
		return chainTimestamp, nil
	}
	return -1, errors.New("API error")
}

//true for nonce, false for time
// return param:
// #param 1: timestamp -> flag false
// #param 2: nonce -> flag true
func (blockCypherClient *BlockCypherClient) GetTimeStampAndNonceByBlockHeight(blockHeight int) (int64, int64, error) {
	time.Sleep(15 * time.Second)

	resp, err := http.Get("https://api.blockcypher.com/v1/btc/main/blocks/" + strconv.Itoa(blockHeight) + "?start=1&limit=1")
	if err != nil {
		return MaxTimeStamp, -1, NewBTCAPIError(APIError, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		blockBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return MaxTimeStamp, -1, NewBTCAPIError(APIError, err)
		}
		block := make(map[string]interface{})
		err = json.Unmarshal(blockBytes, &block)
		if err != nil {
			return MaxTimeStamp, -1, NewBTCAPIError(UnmashallJsonBlockError, errors.New("Can't get nonce or timestamp"))
		}
		nonce, ok := block["nonce"].(float64)
		if !ok {
			return MaxTimeStamp, -1, NewBTCAPIError(WrongTypeError, errors.New("Nonce's type should be float64"))
		}
		timeString, ok := block["time"].(string)
		if !ok {
			return MaxTimeStamp, -1, NewBTCAPIError(WrongTypeError, errors.New("String's type should be string"))
		}
		timeTime, err := time.Parse(time.RFC3339, timeString)
		if err != nil {
			return MaxTimeStamp, -1, NewBTCAPIError(APIError, err)
		}
		timeInt64 := makeTimestamp(timeTime)
		return timeInt64, int64(nonce), nil
	}
	return MaxTimeStamp, -1, NewBTCAPIError(UnExpectedError, errors.New("Can't get nonce or timestamp, status code response "+strconv.Itoa(resp.StatusCode)+" when get block height "+strconv.Itoa(blockHeight)))
}
