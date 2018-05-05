package main

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/cheggaaa/pb"
	"github.com/gocolly/colly"
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

func getFolders(c colly.Collector) (items []*uaitem) {
	err := c.Visit(urlFuck)
	if err != nil {
		fmt.Println(err)
		return nil
	}

	c.OnResponse(func(cr *colly.Response) {
		body := cr.Body
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
	})

	err = c.Post(urlFolders, map[string]string{
		"codasi":     "-1",
		"direccion":  "",
		"expresion":  "",
		"filtro":     "",
		"idmat":      "-1",
		"pendientes": "N",
	})
	if err != nil {
		fmt.Println(err)
		return nil
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

func do(p *pb.ProgressBar, c colly.Collector, item *uaitem) {
	name := formatName(item.name)
	os.MkdirAll(*output+"/"+name, 0777)

	dirs := append([]uaitem{}, uaitem{cod: "-1", name: "./"})
	for inc := 0; inc < len(dirs); inc++ {
		c.OnResponse(func(cr *colly.Response) {
			body := cr.Body

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
		})

		err := c.Post(urlFiles, map[string]string{
			"idmat":          dirs[inc].cod,
			"codasi":         item.cod,
			"expresion":      "",
			"direccion":      "",
			"filtro":         "",
			"pendientes":     "N",
			"fechadesde":     "",
			"fechahasta":     "",
			"busquedarapida": "N",
			"idgrupo":        "",
		})
		if err != nil {
			fmt.Println(err)
			return
		}

		c = *c.Clone()
	}

	dirs = nil
	p.Total = int64(len(item.items))
	p.ShowPercent = true
	p.Prefix(formatName(item.name))

	p.Start()
	for _, i := range item.items {
		download(p, c, item, i)
	}
	p.Finish()
}

func download(p *pb.ProgressBar, c colly.Collector, item, i *uaitem) {
	c.OnResponse(func(cr *colly.Response) {
		from := i.name
		to := formatName(
			fmt.Sprintf("%s/%s/%s", *output, item.name, from),
		)

		os.MkdirAll(path.Dir(to), 0777)

		err := cr.Save(to)
		if err != nil {
			errors = append(errors, err)
		}

		p.Add(1)
	})

	c.Post(urlDownload, map[string]string{
		"identificadores": i.cod,
		"codasis":         item.cod,
	})
}
