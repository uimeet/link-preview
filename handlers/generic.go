package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"go.mau.fi/whatsmeow"
	"golang.org/x/net/html"
)

const (
	StandardMetaTags = iota
	WeChatMP
)

type PreviewHandler interface {
	PreviewContext() *LinkPreviewContext
	Preview() (*LinkPreviewContext, error)
}

func GetPreviewHandler(c *LinkPreviewContext) (PreviewHandler, error) {
	if nil == c {
		return nil, errors.New("bad link preview cxt, nil given")
	}

	if nil == c.Client {
		c.initClient()
	}

	var handler PreviewHandler

	switch c.TargetType {
	case StandardMetaTags:
		handler = &StandardLinkPreview{
			c,
		}
	default:
		return nil, errors.New("unknown target type")
	}

	return handler, nil
}

type HTMLMetaAttr struct {
	Key   string
	Value string
}

type LinkPreviewContext struct {
	TargetType  int    `json:"-"`
	Language    string `json:"-"`
	Title       string `json:"title"`
	Description string `json:"description"`
	ImageURL    string `json:"image"`
	Link        string `json:"website"`

	ImageBytes     []byte                    `json:"-"`
	FinalLink      string                    `json:"-"`
	Client         *http.Request             `json:"-"`
	Parsed         *goquery.Document         `json:"-"`
	UploadResponse *whatsmeow.UploadResponse `json:"-"`
}

func (p *LinkPreviewContext) PreviewContext() *LinkPreviewContext {
	return p
}

func (p *LinkPreviewContext) initClient() {
	if p.Language == "" {
		p.Language = "en-US"
	}
	client, _ := http.NewRequest("GET", p.Link, nil)
	client.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/138.0.0.0 Safari/537.36")
	client.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	client.Header.Set("Accept-Language", fmt.Sprintf("%s,%s;q=0.9", p.Language, strings.Split(p.Language, "-")[0]))
	p.Client = client
}

func (p *LinkPreviewContext) request() error {
	res, err := http.DefaultClient.Do(p.Client)
	if nil != err {
		return err
	}
	defer res.Body.Close()

	p.FinalLink = res.Request.URL.String()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if nil != err {
		return err
	}

	p.Parsed = doc
	return nil
}

func (p *LinkPreviewContext) parseFavicon(node *html.Node) {
	var link string

	for _, attr := range node.Attr {
		switch strings.ToLower(attr.Key) {
		case "href":
			link = attr.Val
			break
		default:
			continue
		}
	}

	if "" == link {
		return
	}

	if strings.HasPrefix("http://", link) || strings.HasPrefix("https://", link) {
		p.ImageURL = link
		return
	}

	parsedURL, _ := url.Parse(p.Link)
	joinedURL := url.URL{
		Scheme: parsedURL.Scheme,
		Host:   parsedURL.Host,
		Path:   link,
	}

	link = joinedURL.String()
	if "" == p.ImageURL {
		p.ImageURL = link
	}
}

func (p *LinkPreviewContext) GetImageBytes() ([]byte, error) {
	if p.ImageBytes != nil {
		fmt.Println("从缓存中获取 ImageBytes")
		return p.ImageBytes, nil
	}
	if p.ImageURL == "" {
		return nil, errors.New("image not found")
	}

	resp, err := http.Get(p.ImageURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 使用 bytes.Buffer 流式读取
	var buf bytes.Buffer
	// 按块复制数据到缓冲区
	_, err = io.Copy(&buf, resp.Body)
	if err != nil {
		return nil, err
	}
	p.ImageBytes = buf.Bytes()
	return p.ImageBytes, nil
}
