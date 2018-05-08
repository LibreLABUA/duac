package main

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

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
	"&#191;", "a",
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
	rdir  = regexp.MustCompile(`<div class="columna2">(.*?)</div>`)
	rname = regexp.MustCompile(`class="nombre" >(.*?)</span>`)
	rid   = regexp.MustCompile(`<div class="columna1">(.*?)</div>`)
)

func do(
	p *pb.ProgressBar, client *fasthttp.Client,
	cookies *cookiejar.CookieJar, item *uaitem,
) {
	mainItem := item
	name := formatName(item.name)
	os.MkdirAll(*output+"/"+name, 0777)

	dirs := append([]uaitem{}, uaitem{cod: "-1", name: "./"})
	for inc := 0; inc < len(dirs); inc++ {
		args := fasthttp.AcquireArgs()
		req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()

		args.Set("idmat", dirs[inc].cod)
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

		fasthttp.ReleaseArgs(args)
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)

		idMatch := rid.FindAllSubmatch(body, -1)
		nameMatch := rname.FindAllSubmatch(body, -1)
		dirMatch := rdir.FindAllSubmatch(body, -1)
		for i := 0; i < len(idMatch); i++ {
		sloop:
			for j := 1; j < len(idMatch[i]); j += 2 {
				cod := string(idMatch[i][j])
				name := string(nameMatch[i][j])
				if len(dirMatch[i][j]) == 0 {
					dirs = append(dirs, uaitem{
						cod: cod, name: fmt.Sprintf("%s/%s", dirs[inc].name, name),
					})
					continue sloop
				}
				item.items = append(item.items, &uaitem{
					cod:  cod,
					name: fmt.Sprintf("%s/%s", dirs[inc].name, name),
				})
			}
		}

		// deleting processed dir
		dirs = dirs[1:]
		inc--
	}
	dirs = nil
	p.Total = int64(len(item.items))
	p.ShowPercent = true
	p.Prefix(formatName(item.name))
	if p.Total != 0 {
		download(p, client, cookies, mainItem)
	}
}

func download(
	p *pb.ProgressBar, client *fasthttp.Client,
	cookies *cookiejar.CookieJar, item *uaitem,
) {
	p.Start()
	for i := range item.items {
		args := fasthttp.AcquireArgs()
		req, res := fasthttp.AcquireRequest(), fasthttp.AcquireResponse()

		args.Set("identificadores", item.items[i].cod)
		args.Set("codasis", item.cod)

		req.SetRequestURI(urlDownload)
		req.Header.SetContentType("application/x-www-form-urlencoded")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,image/apng,*/*;q=0.8")
		req.Header.Set("Accept-Encoding", "gzip")
		req.Header.SetMethod("POST")

		args.WriteTo(req.BodyWriter())
		fasthttp.ReleaseArgs(args)

		from := item.items[i].name
		to := formatName(
			fmt.Sprintf("%s/%s/%s", *output, item.name, from),
		)

		err := doReqFollowRedirects(req, res, client, cookies)
		if err != nil {
			errors = append(errors, err)
		}
		p.Add(1)
		if bytes.Equal(res.Header.ContentType(), []byte("application/zip")) {
			if !strings.Contains(
				path.Ext(to), ".zip",
			) {
				to += ".zip"
			}
		}
		os.MkdirAll(path.Dir(to), 0777)

		file, err := os.Create(to)
		if err != nil {
			errors = append(errors, err)
			return
		}

		res.BodyWriteTo(file)
		file.Close()

		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(res)
	}
	p.Finish()
}
