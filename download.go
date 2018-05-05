package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/cheggaaa/pb"
	"github.com/erikdubbelboer/fasthttp"
	"github.com/themester/fcookiejar"
)

type uaitem struct {
	cod   string
	name  string
	items []*uaitem
}

var (
	rcod  = regexp.MustCompile(`data-codasi="(.*?)"`)
	rasig = regexp.MustCompile(`<span class="asi">(.*?)</span>`)
)

func getFolders(client *fasthttp.Client, cookies *cookiejar.CookieJar) (items []*uaitem) {
	args := fasthttp.AcquireArgs()
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseArgs(args)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	req.SetRequestURI(urlFuck)
	err := doReqFollowRedirects(req, res, client, cookies)
	if err != nil {
		fmt.Println(err)
		return nil
	}
	req.Reset()
	res.Reset()

	args.Set("codasi", "-1")
	args.Set("direccion", "")
	args.Set("expresion", "")
	args.Set("filtro", "")
	args.Set("idmat", "-1")
	args.Set("pendientes", "N")

	args.WriteTo(req.BodyWriter())

	// preparing request
	req.SetRequestURI(urlFolders)
	req.Header.SetMethod("POST")

	err = doReqFollowRedirects(req, res, client, cookies)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	body := res.Body()
	codeMatch := rcod.FindAllSubmatch(body, -1)
	nameMatch := rasig.FindAllSubmatch(body, -1)
	// ignoring first
	for i := 0; i < len(codeMatch) && i < len(nameMatch); i++ {
		for j := 1; j < len(codeMatch[i]) && j < len(nameMatch[i]); j += 2 {
			items = append(items, &uaitem{
				cod:  string(codeMatch[i+1][j]),
				name: string(nameMatch[i][j]),
			})
		}
	}

	return
}

var htmlescapecodes = []string{
	"&#193;", "A",
	"&#241;", "ny",
	"&#225;", "a",
	"&#233;", "e",
	"&#237;", "i",
	"&#243;", "o",
	"&#250;", "u",
	"&#209;", "NY",
	"&#193;", "A",
	"&#201;", "E",
	"&#205;", "i",
	"&#211;", "O",
	"&#218;", "U",
}

var replacer = strings.NewReplacer(htmlescapecodes...)

func formatName(s string) string {
	return replacer.Replace(s)
}

var (
	rname = regexp.MustCompile(`class="nombre" >(.*?)</span>`)
	rid   = regexp.MustCompile(`<div class="columna1">(.*?)</div>`)
)

func do(
	p *pb.ProgressBar, client *fasthttp.Client,
	cookies *cookiejar.CookieJar, item *uaitem,
) {
	args := fasthttp.AcquireArgs()
	req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
	defer fasthttp.ReleaseArgs(args)
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(res)

	args.Set("idmat", "-1")
	args.Set("codasi", item.cod)
	args.Set("expresion", "")
	args.Set("direccion", "")
	args.Set("filtro", "")
	args.Set("pendientes", "N")
	args.Set("fechadesde", "")
	args.Set("fechahasta", "")
	args.Set("busquedarapida", "N")
	args.Set("idgrupo", "")

	args.WriteTo(req.BodyWriter())

	req.Header.SetContentType("application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.SetMethod("POST")
	req.SetRequestURI(urlFiles)

	err := doReqFollowRedirects(req, res, client, cookies)
	if err != nil {
		fmt.Println(err)
		return
	}
	body := res.Body()

	name := formatName(item.name)
	os.MkdirAll(*output+"/"+name, 0777)

	idMatch := rid.FindAllSubmatch(body, -1)
	nameMatch := rname.FindAllSubmatch(body, -1)
	for i := 0; i < len(idMatch); i++ {
		for j := 1; j < len(idMatch[i]); j++ {
			item.items = append(item.items, &uaitem{
				cod:  string(idMatch[i][j]),
				name: string(nameMatch[i][j]),
			})
		}
	}
	p.Total = int64(len(item.items))
	p.ShowPercent = true
	p.Prefix(formatName(item.name))
	if p.Total != 0 {
		download(p, client, cookies, item)
	}
}

func download(
	p *pb.ProgressBar, client *fasthttp.Client,
	cookies *cookiejar.CookieJar, item *uaitem,
) {
	var wg sync.WaitGroup
	p.Start()
	for n := range item.items {
		wg.Add(1)
		go func(i int) {
			dst := ""
			defer wg.Done()

			args := fasthttp.AcquireArgs()
			req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()
			defer fasthttp.ReleaseArgs(args)
			defer fasthttp.ReleaseRequest(req)
			defer fasthttp.ReleaseResponse(res)

			args.Set("identificadores", item.items[i].cod)
			args.Set("codasis", item.cod)

			req.SetRequestURI(urlDownload)
			req.Header.SetContentType("application/x-www-form-urlencoded")
			req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
			req.Header.Set("Accept-Encoding", "gzip")
			req.Header.SetMethod("POST")

			args.WriteTo(req.BodyWriter())

			from := item.items[i].name
			to := formatName(
				fmt.Sprintf("%s/%s/%s", *output, item.name, from),
			)

			err := doReqFollowRedirects(req, res, client, cookies)
			if err != nil {
				p.Add(1)
				errors = append(errors, err)
			}
			if bytes.Equal(res.Header.ContentType(), []byte("application/zip")) {
				if !strings.Contains(
					path.Ext(to), ".zip",
				) {
					dst = to
					to += ".zip"
				}
			}

			ioutil.WriteFile(to, res.Body(), 0644)
			if dst != "" {
				// Does not work well with the f*cking UA rare compressed files
				//uncompress(to, dst)
			}
			p.Add(1)
		}(n)
	}
	wg.Wait()
	p.Finish()
}

func uncompress(src, dst string) {
	r, err := zip.OpenReader(src)
	if err != nil {
		errors = append(errors, err)
		return
	}
	defer r.Close()

	os.MkdirAll(dst, 0777)

	// Iterate through the files in the archive,
	for _, f := range r.File {
		srcFile, err := f.Open()
		if err != nil {
			errors = append(errors, err)
			continue
		}

		dstFile, err := os.Create(
			fmt.Sprintf("%s/%s", dst, formatName(f.Name)),
		)
		if err != nil {
			errors = append(errors, err)
			goto next
		}

		_, err = io.Copy(dstFile, srcFile)
		if err != nil {
			errors = append(errors, err)
		}
	next:
		srcFile.Close()
		dstFile.Close()
	}
}
