// Package soap provides a SOAP HTTP client.
package soap

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/sethgrid/pester"
)

// A RoundTripper executes a request passing the given req as the SOAP
// envelope body. The HTTP response is then de-serialized onto the resp
// object. Returns error in case an error occurs serializing req, making
// the HTTP request, or de-serializing the response.
type RoundTripper interface {
	RoundTrip(req, resp Message) error
}

// Message is an opaque type used by the RoundTripper to carry XML
// documents for SOAP.
type Message interface{}

// Header is an opaque type used as the SOAP Header element in requests.
type Header interface{}

// AuthHeader is a Header to be encoded as the SOAP Header element in
// requests, to convey credentials for authentication.
type AuthHeader struct {
	Namespace string `xml:"xmlns:ns,attr"`
	Username  string `xml:"ns:username"`
	Password  string `xml:"ns:password"`
}

// UseResiliencyLibrary sets a new pester client with built in request
// resiliency.
func (c *Client) UseResiliencyLibrary() {
	c.ResilientClient = pester.New()
}

type HttpClient interface {
	//func (c *Client) Do(req *http.Request) (resp *http.Response, err error) {
	//func (c *Client) Do(req *Request) (*Response, error) {
	Do(req *http.Request) (*http.Response, error)
}

// Client is a SOAP client.
type Client struct {
	URL         string // URL of the server
	Namespace   string // SOAP Namespace
	Envelope    string // Optional SOAP Envelope
	Header      Header // Optional SOAP Header
	ContentType string // Optional Content-Type (default text/xml)
	//ResilientClient *pester.Client      // Optional pestor client
	//Config          *http.Client        // Optional HTTP client
	ResilientClient HttpClient          // Optional pestor client
	Config          HttpClient          // Optional HTTP client
	Pre             func(*http.Request) // Optional hook to modify outbound requests
}

// RoundTrip implements the RoundTripper interface.
func (c *Client) RoundTrip(in, out Message) error {
	req := &Envelope{
		EnvelopeAttr: c.Envelope,
		NSAttr:       c.Namespace,
		Header:       c.Header,
		Body:         Body{Message: in},
	}
	if req.EnvelopeAttr == "" {
		req.EnvelopeAttr = "http://schemas.xmlsoap.org/soap/envelope/"
	}
	if req.NSAttr == "" {
		req.NSAttr = c.URL
	}
	var b bytes.Buffer
	err := xml.NewEncoder(&b).Encode(req)
	if err != nil {
		return err
	}
	ct := c.ContentType
	if ct == "" {
		ct = "text/xml"
	}

	var cli HttpClient
	if c.ResilientClient != nil {
		cli = c.ResilientClient
	} else {
		cli = c.Config
		if cli == nil {
			cli = http.DefaultClient
		}
	}

	r, err := http.NewRequest("POST", c.URL, &b)
	if err != nil {
		return err
	}
	r.Header.Set("Content-Type", ct)
	if c.Pre != nil {
		c.Pre(r)
	}
	resp, err := cli.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// read only the first Mb of the body in error case
		limReader := io.LimitReader(resp.Body, 1024*1024)
		body, _ := ioutil.ReadAll(limReader)
		return fmt.Errorf("%q: %q", resp.Status, body)
	}
	return xml.NewDecoder(resp.Body).Decode(out)
}

// Envelope is a SOAP envelope.
type Envelope struct {
	XMLName      xml.Name `xml:"SOAP-ENV:Envelope"`
	EnvelopeAttr string   `xml:"xmlns:SOAP-ENV,attr"`
	NSAttr       string   `xml:"xmlns:ns,attr"`
	Header       Message  `xml:"SOAP-ENV:Header"`
	Body         Body
}

// Body is the body of a SOAP envelope.
type Body struct {
	XMLName xml.Name `xml:"SOAP-ENV:Body"`
	Message Message
}
