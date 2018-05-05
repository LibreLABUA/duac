package main

import (
	"fmt"
	"regexp"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/erikdubbelboer/fasthttp"
	"github.com/themester/fcookiejar"
)

var (
	// regex execution parameter
	regexep = regexp.MustCompile(`name="execution"\svalue="(.*?)"`)
)

func login(user, pass string) (*fasthttp.Client, *cookiejar.CookieJar, error) {
	client := &fasthttp.Client{
		Name:                "Pakillo 4.20",
		MaxResponseBodySize: int(datasize.MB) * 100, // 100 mb of download files
	}

	// Be patient... UA's webpage is written in C#
	status, body, err := client.GetTimeout(nil, urlNormal, time.Second*15)
	if err != nil {
		return nil, nil, err
	}
	if status != 200 {
		return nil, nil, fmt.Errorf("Unexpected response code: %d<>200", status)
	}

	// getting execution parameter
	execp := regexep.FindSubmatch(body)
	if len(execp) == 0 {
		return nil, nil, fmt.Errorf("error getting login parameters...")
	}

	// reusing body as much as possible
	// submatch is the latest parameter so we take len-1
	body = append(body[:0], execp[len(execp)-1]...)
	execp = nil

	// getting request, response and arguments for post request
	args := fasthttp.AcquireArgs()
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseArgs(args)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	// compression is better. Compress your life :')
	req.Header.Add("Accept-Encoding", "gzip")
	req.Header.SetContentType("application/x-www-form-urlencoded")
	req.Header.SetMethod("POST")

	req.SetRequestURI(urlLogin)

	// setting parameters for post request
	args.Set("_eventId", "submit")
	args.Set("username", user)
	args.Set("password", pass)
	args.Set("geolocation", "")
	args.SetBytesV("execution", body)

	// writting post arguments to request body
	args.WriteTo(req.BodyWriter())

	// creating cookieJar object
	cookies := cookiejar.AcquireCookieJar()

	// make requests xd
	err = doReqFollowRedirects(req, res, client, cookies)
	if err != nil {
		return nil, nil, err
	}

	// getting new cookies
	cookies.ResponseCookies(res)

	// 401 code if password is incorrect
	if status == 401 {
		err = fmt.Errorf("Incorrect password")
	}

	return client, cookies, err
}
