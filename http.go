package main

import (
	"fmt"

	"github.com/erikdubbelboer/fasthttp"
	"github.com/themester/fcookiejar"
)

func doReqFollowRedirects(
	req *fasthttp.Request, res *fasthttp.Response,
	client *fasthttp.Client, cookies *cookiejar.CookieJar) (err error) {

	var url, body []byte
	var referer string
	// use compression!!!11!
	req.Header.Add("Accept-Encoding", "gzip")
	for {
		if referer != "" {
			req.Header.Add("Referer", referer)
		}
		// writting cookies to request
		cookies.WriteToRequest(req)

		err = client.Do(req, res)
		if err != nil {
			goto end
		}
		// reading cookies from the response
		cookies.ResponseCookies(res)

		status := res.Header.StatusCode()
		if status != fasthttp.StatusMovedPermanently &&
			status != fasthttp.StatusFound &&
			status != fasthttp.StatusSeeOther &&
			status != fasthttp.StatusTemporaryRedirect &&
			status != fasthttp.StatusPermanentRedirect {
			break
		}

		referer, url = string(url), res.Header.Peek("Location")
		if len(url) == 0 {
			err = fmt.Errorf("Status code is redirect (%d) but no one location have been provided", status)
			goto end
		}
		req.Reset()
		res.Reset()

		req.SetRequestURIBytes(url)
		req.Header.SetMethod("GET")
		req.Header.Add("Accept-Encoding", "gzip")
	}
	if res.Header.StatusCode() == 401 {
		err = fmt.Errorf("Incorrect password")
		goto end
	}

	body = res.Body()
	if len(res.Header.Peek("Content-Encoding")) != 0 {
		// gunzipping
		fasthttp.AppendGunzipBytes(body, body)
		res.SetBody(body)
	}

end:
	return err
}
