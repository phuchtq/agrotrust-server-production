package walruspkg

import (
	"log"

	walrus_go "github.com/namihq/walrus-go"
)

type walrusProvider struct {
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
			client:    walrusClient,
			errLogger: errLogger,
		}
	}

	return _walrusProvider
}

// FetchBytesImage implements IWalrusProvider.
func (w *walrusProvider) FetchBytesImage(blobID string) ([]byte, error) {

	// res, err := w.client.Read(blobID, nil)
	// if err != nil {
	// 	w.errLogger.Println(err.Error())
	// }

	return nil, nil
}
