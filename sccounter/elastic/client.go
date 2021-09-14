package elastic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"time"

	logger "github.com/ElrondNetwork/elrond-go-logger"
	"github.com/ElrondNetwork/elrond-tools-go/sccounter/config"
	"github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/tidwall/gjson"
)

var (
	log                  = logger.GetOrCreate("elastic")
	httpStatusesForRetry = []int{429, 502, 503, 504}
)

const stepDelayBetweenRequests = 500 * time.Millisecond

type esClient struct {
	client *elasticsearch.Client

	// countScroll is used to be incremented after each scroll so the scroll duration is different each time,
	// bypassing any possible caching based on the same request
	countScroll int
}

// NewElasticClient will create a new instance of an esClient
func NewElasticClient(cfg config.ElasticInstanceConfig) (*esClient, error) {
	elasticClient, err := elasticsearch.NewClient(elasticsearch.Config{
		Addresses:     []string{cfg.URL},
		Username:      cfg.Username,
		Password:      cfg.Password,
		RetryOnStatus: httpStatusesForRetry,
		RetryBackoff: func(i int) time.Duration {
			// A simple exponential delay
			d := time.Duration(math.Exp2(float64(i))) * time.Second
			log.Info("elastic: retry backoff", "attempt", i, "sleep duration", d)
			return d
		},
		MaxRetries: 5,
	})
	if err != nil {
		return nil, err
	}

	return &esClient{
		client:      elasticClient,
		countScroll: 0,
	}, nil
}

// DoScrollRequestAllDocuments will perform a documents request using scroll api
func (esc *esClient) DoScrollRequestAllDocuments(
	index string,
	body []byte,
	handlerFunc func(responseBytes []byte) error,
) error {
	esc.countScroll++
	res, err := esc.client.Search(
		esc.client.Search.WithSize(9000),
		esc.client.Search.WithScroll(10*time.Minute+time.Duration(esc.countScroll)*time.Millisecond),
		esc.client.Search.WithContext(context.Background()),
		esc.client.Search.WithIndex(index),
		esc.client.Search.WithBody(bytes.NewBuffer(body)),
	)
	if err != nil {
		return err
	}

	bodyBytes, err := getBytesFromResponse(res)
	if err != nil {
		return err
	}

	err = handlerFunc(bodyBytes)
	if err != nil {
		return err
	}

	scrollID := gjson.Get(string(bodyBytes), "_scroll_id")
	return esc.iterateScroll(scrollID.String(), handlerFunc)
}

// DoBulkRequest will do a bulk of request to elastic server
func (esc *esClient) DoBulkRequest(buff *bytes.Buffer, index string) error {
	reader := bytes.NewReader(buff.Bytes())

	res, err := esc.client.Bulk(
		reader,
		esc.client.Bulk.WithIndex(index),
	)
	if err != nil {
		return err
	}
	if res.IsError() {
		return fmt.Errorf("%s", res.String())
	}

	defer closeBody(res)

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}

	bulkResponse := &bulkRequestResponse{}
	err = json.Unmarshal(bodyBytes, bulkResponse)
	if err != nil {
		return err
	}

	if bulkResponse.Errors {
		return extractErrorFromBulkResponse(bulkResponse)
	}

	return nil
}

func (esc *esClient) iterateScroll(
	scrollID string,
	handlerFunc func(responseBytes []byte) error,
) error {
	if scrollID == "" {
		return nil
	}
	defer func() {
		err := esc.clearScroll(scrollID)
		if err != nil {
			log.Warn("cannot clear scroll", "error", err)
		}
	}()

	for {
		scrollBodyBytes, errScroll := esc.getScrollResponse(scrollID)
		if errScroll != nil {
			return errScroll
		}

		numberOfHits := gjson.Get(string(scrollBodyBytes), "hits.hits.#")
		if numberOfHits.Int() < 1 {
			return nil
		}
		err := handlerFunc(scrollBodyBytes)
		if err != nil {
			return err
		}

		time.Sleep(stepDelayBetweenRequests)
	}
}

func (esc *esClient) getScrollResponse(scrollID string) ([]byte, error) {
	esc.countScroll++
	res, err := esc.client.Scroll(
		esc.client.Scroll.WithScrollID(scrollID),
		esc.client.Scroll.WithScroll(2*time.Minute+time.Duration(esc.countScroll)*time.Millisecond),
	)
	if err != nil {
		return nil, err
	}

	return getBytesFromResponse(res)
}

func (esc *esClient) clearScroll(scrollID string) error {
	resp, err := esc.client.ClearScroll(
		esc.client.ClearScroll.WithScrollID(scrollID),
	)
	if err != nil {
		return err
	}
	defer closeBody(resp)

	if resp.IsError() && resp.StatusCode != http.StatusNotFound {
		return fmt.Errorf("error response: %s", resp)
	}

	return nil
}

// IsInterfaceNil returns true if there is no value under the interface
func (esc *esClient) IsInterfaceNil() bool {
	return esc == nil
}

func getBytesFromResponse(res *esapi.Response) ([]byte, error) {
	if res.IsError() {
		return nil, fmt.Errorf("error response: %s", res)
	}
	defer closeBody(res)

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return bodyBytes, nil
}

func closeBody(res *esapi.Response) {
	if res != nil && res.Body != nil {
		_ = res.Body.Close()
	}
}