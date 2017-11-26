package gbfs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	pb_dur "github.com/golang/protobuf/ptypes/duration"
	pb_ts "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/patrickmn/go-cache"
	"github.com/skidder/gbfs-go/gbfs/proto"
)

const (
	completeGBFSLabel       = "complete"
	languagesLabel          = "languages"
	autoDiscoveryLabel      = "autodiscovery"
	stationStatusLabel      = "station_status"
	stationInformationLabel = "station_information"
)

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Client is a client for GBFS feeds.
type Client struct {
	feedURL string
	cache   *cache.Cache
}

// NewClient creates a client capable of retrieve GBFS Feed data.
func NewClient(feedURL string) *Client {
	return &Client{
		feedURL: feedURL,
		cache:   cache.New(10*time.Second, 10*time.Minute),
	}
}

// GetSupportedLanguages returns the set of languages supported by the feeds
// advertised in the auto-discovery data.
func (g *Client) GetSupportedLanguages() ([]string, error) {
	if message, found := g.cache.Get(languagesLabel); found {
		return message.([]string), nil
	}
	autoDiscoveryMessage, err := g.GetAutoDiscoveryMessage()
	if err != nil {
		return nil, err
	}
	languages := make([]string, 0)
	for language := range autoDiscoveryMessage.Data {
		languages = append(languages, language)
	}
	g.cache.Set(languagesLabel, languages, time.Duration(autoDiscoveryMessage.Ttl)*time.Second)
	return languages, nil
}

// GetCompleteGBFSMessage returns a GBFS message with the complete set of data for the given language.
func (g *Client) GetCompleteGBFSMessage(language string) (*proto.CompleteGBFSMessage, error) {
	if message, found := g.cache.Get(completeGBFSLabel); found {
		return message.(*proto.CompleteGBFSMessage), nil
	}
	target := &proto.CompleteGBFSMessage{}
	var err error
	target.AutoDiscoveryMessage, err = g.GetAutoDiscoveryMessage()
	if err != nil {
		return nil, err
	}
	target.StationStatusMessage, err = g.GetStationStatus(language)
	if err != nil {
		return nil, err
	}
	target.StationInformationMessage, err = g.GetStationInformation(language)
	if err != nil {
		return nil, err
	}
	return target, nil
}

func (g *Client) GetAutoDiscoveryMessage() (*proto.AutoDiscoveryMessage, error) {
	if message, found := g.cache.Get(autoDiscoveryLabel); found {
		return message.(*proto.AutoDiscoveryMessage), nil
	}

	target := &proto.AutoDiscoveryMessage{}
	err := download(g.feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	g.cache.Set(autoDiscoveryLabel, target, time.Duration(target.Ttl)*time.Second)
	return target, nil
}

// GetStationStatus returns the status of all stations using the specified language code.
func (g *Client) GetStationStatus(language string) (*proto.StationStatusMessage, error) {
	if message, found := g.cache.Get(stationStatusLabel); found {
		return message.(*proto.StationStatusMessage), nil
	}

	feedURL, err := g.getChildFeedURL(language, stationStatusLabel)
	if err != nil {
		return nil, err
	}

	target := &proto.StationStatusMessage{}
	err = download(feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	for _, station := range target.Data.Stations {
		station.LastReportedTimestamp = &pb_ts.Timestamp{Seconds: int64(station.LastReported)}
	}
	g.cache.Set(stationStatusLabel, target, time.Duration(target.Ttl)*time.Second)
	return target, nil
}

// GetStationInformation returns descriptive information for all stations using the specified language code.
func (g *Client) GetStationInformation(language string) (*proto.StationInformationMessage, error) {
	if message, found := g.cache.Get(stationInformationLabel); found {
		return message.(*proto.StationInformationMessage), nil
	}

	feedURL, err := g.getChildFeedURL(language, stationInformationLabel)
	if err != nil {
		return nil, err
	}

	target := &proto.StationInformationMessage{}
	err = download(feedURL, target)
	if err != nil {
		return nil, err
	}
	target.LastUpdatedTimestamp = &pb_ts.Timestamp{Seconds: int64(target.LastUpdated)}
	target.TtlDuration = &pb_dur.Duration{Seconds: int64(target.Ttl)}
	g.cache.Set(stationInformationLabel, target, time.Duration(target.Ttl)*time.Second)
	return target, nil
}

// Locates the child-feed URL from the auto-discovery data using the supplied language.
func (g *Client) getChildFeedURL(language string, feedName string) (string, error) {
	autoDiscoveryMessage, err := g.GetAutoDiscoveryMessage()
	if err != nil {
		return "", err
	}
	languageEntry, found := autoDiscoveryMessage.Data[language]
	if !found {
		return "", fmt.Errorf("Language not found in auto-discovery response: %s", language)
	}
	var feedURL string
	for _, feed := range languageEntry.Feeds {
		if feedName == feed.Name {
			feedURL = feed.Url
		}
	}
	if feedURL == "" {
		return "", fmt.Errorf("Feed URL not found for feed name: %s", feedName)
	}
	return feedURL, nil
}

// Download data from the supplied URL and parse as JSON into the given structure.
func download(url string, target interface{}) error {
	r, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	err = json.NewDecoder(r.Body).Decode(target)
	if err != nil {
		return err
	}
	return nil
}
