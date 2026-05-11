package walruspkg

import (
	"fmt"
	"io"
	"log"
	"net/http"

	walrus_go "github.com/namihq/walrus-go"
)

type walrusProvider struct {
	url       string
	client    *walrus_go.Client
	errLogger *log.Logger
}

var _walrusProvider *walrusProvider

type IWalrusProvider interface {
	FetchBytesImage(blobID string) ([]byte, error)
}

func InitializeWalrusProvider(errLogger *log.Logger) IWalrusProvider {
	if _walrusProvider == nil {
		var walrusClient = walrus_go.NewClient()

		walrusClient.AggregatorURL = []string{
			"https://walrus.space",
			"https://nodes.guru",
		}

		walrusClient.PublisherURL = []string{
			"https://walrus.space",
		}

		_walrusProvider = &walrusProvider{
			url:       "https://aggregator.walrus-testnet.walrus.space/v1/blobs/",
			client:    walrusClient,
			errLogger: errLogger,
		}
	}

	return _walrusProvider
}

// FetchBytesImage implements IWalrusProvider.
func (w *walrusProvider) FetchBytesImage(blobID string) ([]byte, error) {
	var postUrl string = w.url + blobID

	resp, err := http.Get(postUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch blob: HTTP %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}
