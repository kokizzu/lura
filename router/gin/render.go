// SPDX-License-Identifier: Apache-2.0

package gin

import (
	"io"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/luraproject/lura/v2/config"
	"github.com/luraproject/lura/v2/encoding"
	"github.com/luraproject/lura/v2/proxy"
)

// Render defines the signature of the functions to be use for the final response
// encoding and rendering
type Render func(*gin.Context, *proxy.Response)

// NEGOTIATE defines the value of the OutputEncoding for the negotiated render
const NEGOTIATE = "negotiate"
const XML = "xml"
const YAML = "yaml"

var (
	mutex          = &sync.RWMutex{}
	renderRegister = map[string]Render{
		encoding.STRING:   stringRender,
		encoding.JSON:     jsonRender,
		encoding.NOOP:     noopRender,
		"json-collection": jsonCollectionRender,
		XML:               xmlRender,
		YAML:              yamlRender,
	}
)

func init() {
	// the negotiated render must be registered at the init function in order
	// to avoid a cyclical dependency
	renderRegister[NEGOTIATE] = negotiatedRender
}

// RegisterRender allows clients to register their custom renders
func RegisterRender(name string, r Render) {
	mutex.Lock()
	renderRegister[name] = r
	mutex.Unlock()
}

func getRender(cfg *config.EndpointConfig) Render {
	fallback := jsonRender
	if len(cfg.Backend) == 1 {
		fallback = getWithFallback(cfg.Backend[0].Encoding, fallback)
	}

	if cfg.OutputEncoding == "" {
		return fallback
	}

	return getWithFallback(cfg.OutputEncoding, fallback)
}

func getWithFallback(key string, fallback Render) Render {
	mutex.RLock()
	r, ok := renderRegister[key]
	mutex.RUnlock()
	if !ok {
		return fallback
	}
	return r
}

func negotiatedRender(c *gin.Context, response *proxy.Response) {
	switch c.NegotiateFormat(gin.MIMEJSON, gin.MIMEPlain, gin.MIMEXML, gin.MIMEYAML) {
	case gin.MIMEXML:
		getWithFallback(XML, jsonRender)(c, response)
	case gin.MIMEPlain, gin.MIMEYAML:
		getWithFallback(YAML, jsonRender)(c, response)
	default:
		getWithFallback(encoding.JSON, jsonRender)(c, response)
	}
}

func stringRender(c *gin.Context, response *proxy.Response) {
	status := c.Writer.Status()

	if response == nil {
		c.String(status, "")
		return
	}
	d, ok := response.Data["content"]
	if !ok {
		c.String(status, "")
		return
	}
	msg, ok := d.(string)
	if !ok {
		c.String(status, "")
		return
	}
	c.String(status, msg)
}

func jsonRender(c *gin.Context, response *proxy.Response) {
	status := c.Writer.Status()
	if response == nil {
		c.JSON(status, emptyResponse)
		return
	}
	c.JSON(status, response.Data)
}

func jsonCollectionRender(c *gin.Context, response *proxy.Response) {
	status := c.Writer.Status()
	if response == nil {
		c.JSON(status, []struct{}{})
		return
	}
	col, ok := response.Data["collection"]
	if !ok {
		c.JSON(status, []struct{}{})
		return
	}
	c.JSON(status, col)
}

func xmlRender(c *gin.Context, response *proxy.Response) {
	status := c.Writer.Status()
	if response == nil {
		c.XML(status, nil)
		return
	}
	d, ok := response.Data["content"]
	if !ok {
		c.XML(status, nil)
		return
	}
	c.XML(status, d)
}

func yamlRender(c *gin.Context, response *proxy.Response) {
	status := c.Writer.Status()
	if response == nil {
		c.YAML(status, emptyResponse)
		return
	}
	c.YAML(status, response.Data)
}

func noopRender(c *gin.Context, response *proxy.Response) {
	if response == nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	for k, vs := range response.Metadata.Headers {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(response.Metadata.StatusCode)
	if response.Io == nil {
		return
	}
	io.Copy(c.Writer, response.Io)
}

var emptyResponse = gin.H{}
