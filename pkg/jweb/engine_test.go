package jweb

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"sort"
	"testing"

	"github.com/gin-gonic/gin"
)

type MyContext struct {
	c *gin.Context
}

func (c *MyContext) SetGinContext(ctx *gin.Context) {
	c.c = ctx
}

func (c *MyContext) GetGinContext() *gin.Context {
	return c.c
}

func (c *MyContext) ResponseParseParamsFieldFail(path string, field string, value string, err error) {
	c.c.JSON(300, map[string]interface{}{
		"STATUS": "FAIL",
		"MSG":    err.Error(),
	})
}

func (c *MyContext) ResponseFail(code int, m map[string]interface{}) {
	c.c.JSON(code, map[string]interface{}{
		"STATUS": "FAIL",
		"MSG":    m,
	})
}

func (c *MyContext) ResponseOK(message interface{}) {
	c.c.JSON(http.StatusOK, map[string]interface{}{
		"STATUS": "OK",
		"MSG":    message,
	})
}

// returns := reflect.ValueOf(fa.Fun).Call([]reflect.Value{reflect.ValueOf(req.GetClient().PlayerID), reflect.ValueOf(fa.Arg)})
//
// fmt.Printf("res:%+v\n", returns)
//
// msgidinterface := returns[0].Interface()
func TestEngine(t *testing.T) {

	gine := gin.Default()
	gine.GET("/", func(c *gin.Context) {
		// 测试参数风格
		buf, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}

		if len(buf) == 0 { // 如果body没有参数，则参数来自url
			fmt.Printf("body params len:%v\n", len(buf))
			fmt.Printf("url params:%+v\n", c.Request.URL.Query())
		} else { // 如果body有参数，则用body的参数json反序列化
			fmt.Printf("find body params:%+v\n", string(buf))
		}
		c.JSON(200, "ok")
	})
	gine.POST("/", func(c *gin.Context) {
		buf, err := io.ReadAll(c.Request.Body)
		if err != nil {
			panic(err)
		}

		if len(buf) == 0 {
			fmt.Printf("body params len:%v\n", len(buf))
			fmt.Printf("url params:%+v\n", c.Request.URL.Query())
		} else {
			fmt.Printf("find body params:%+v\n", string(buf))
		}
		c.JSON(200, "ok")
	})
	gine.Run(":5002")
	return

	e := NewEngine(":5001", func() Context {
		return new(MyContext)
	})
	e.Use(func(c *MyContext) {
		ip := c.GetGinContext().RemoteIP()
		fmt.Printf("receive ip(%v) msg\n", ip)
	})
	grp1 := e.Group("/group1")
	{
		grp1.Get("/", "group1首页", func(c *MyContext) {
			fmt.Printf("receive group1 request\n")
			c.ResponseOK("test group1")
		})

		type TestParam struct {
			RoleID  int    `json:"role_id" desc:"角色id"`
			Title   string `json:"title"`
			Content string `json:"content"`
			Items   string `json:"items"`
		}
		grp1.GetWithStructParams("test", "测试", TestParam{}, func(c *MyContext) {
			fmt.Printf("receive group1/test request\n")
			c.ResponseOK("test group1")
		})

		grp2 := grp1.Group("mail")
		{
			type TestParam1 struct {
				RoleID  int    `json:"role_id" desc:"角色id"`
				Title   string `json:"mail_title" desc:"邮件标题"`
				Content string `json:"mail_content"`
				Items   string `json:"items"`
			}
			grp2.GetWithStructParams("add", "添加邮件", TestParam1{}, func(c *MyContext, params *TestParam1) {
				fmt.Printf("receive group1/mail/add request:%+v\n", params)
				c.ResponseOK("test group1")
			})
		}
	}

	e.Get("/index", "首页", func(c *MyContext) {
		fmt.Printf("receive index request\n")
		c.ResponseFail(404, map[string]interface{}{
			"reason": "not found",
			"msg":    "invalid request",
		})
	})

	treeMap := &Tree{e.TravelGroupTree()}

	e.Get("/doc", "获取说明文档", func(c *MyContext) {
		tplText := `
			<!DOCTYPE html>
			<html lang="en">
			<head>
				<meta charset="UTF-8">
				<title>welcome</title>
			
				<div align="center">
					<table border = "1" align="center">
					{{ range $idx, $path := .Paths }}
						<tr>
						<th colspan="4" style="background-color: green">{{ $path.Route.Desc }}</th>
						</tr>
						<tr>
						<td>请求路径</td>
						<th colspan="3">{{ $path.Path }}</th>
						</tr>
						<tr>
						<td>方法</td>
						<th colspan="3">{{ $path.Route.Method }}</th>
						</tr>
						{{ if $path.Route.HasFields }}
						{{ range $idx1, $field := $path.Route.JsonStructDesc }}
							<tr>
							<td>{{ printf "参数%d" $idx1 }}</td>
							<td>{{printf "%s" $field.Name }}</td>
							<td>{{printf "%s" $field.Type }}</td>
							<td>{{printf "%s" $field.Desc }}</td>
							</tr>
						{{ end }}
						{{ else }}
							<tr><th colspan="4">无需参数</th>
						{{ end }}
						<tr><th colspan="4"><hr></th></tr>
					{{ end }}
					</table>
				</div>
			
			</head>
			<body>
			
			</body>
			</html>
		`
		tmpl, err := template.New("html_test").Parse(tplText)
		if err != nil {
			panic(err)
		}
		err = tmpl.Execute(c.GetGinContext().Writer, treeMap)
		if err != nil {
			panic(err)
		}
	})

	for k, v := range e.TravelGroupTree() {
		fmt.Printf("path:%v, params:%v\n", k, v.JsonStructDesc())
	}

	e.Run()
}

type Tree struct {
	t map[string]*RouteInfo
}

type Path struct {
	Path  string
	Route *RouteInfo
}

func (t *Tree) Paths() []*Path {
	list := make([]*Path, 0)
	for k, v := range t.t {
		list = append(list, &Path{Path: k, Route: v})
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].Path < list[j].Path
	})
	return list
}
