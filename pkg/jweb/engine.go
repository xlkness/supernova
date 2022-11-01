package jweb

import (
	"reflect"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.ReleaseMode)
}

// type HandlerFunc func(ctx Context)
type HandlerFunc interface{}

type RouterGroup struct {
	basePath      string
	group         *gin.RouterGroup
	GroupRoutes   map[string]*RouterGroup
	Routes        map[string]*RouteInfo
	newContextFun func() Context
}

func newRouterGroup(group *gin.RouterGroup, newContextFun func() Context) *RouterGroup {
	return &RouterGroup{
		basePath:      group.BasePath(),
		group:         group,
		newContextFun: newContextFun,
		GroupRoutes:   make(map[string]*RouterGroup),
		Routes:        make(map[string]*RouteInfo),
	}
}

func (g *RouterGroup) Use(middleware ...HandlerFunc) {
	g.group.Use(getGinHandlerFun(g.newContextFun, nil, middleware...))
}

func (g *RouterGroup) Group(path string, handlers ...HandlerFunc) *RouterGroup {
	grp := g.group.Group(path, getGinHandlerFun(g.newContextFun, nil, handlers...))
	grp1 := newRouterGroup(grp, g.newContextFun)
	g.GroupRoutes[path] = grp1
	return grp1
}

func (g *RouterGroup) Get(path string, desc string, handlers ...HandlerFunc) gin.IRoutes {
	return g.GetWithStructParams(path, desc, nil, handlers...)
}

func (g *RouterGroup) GetWithStructParams(path string, desc string, structTemplate interface{}, handlers ...HandlerFunc) gin.IRoutes {
	g.Routes[path] = &RouteInfo{Desc: desc, Method: "GET", StructTemplate: structTemplate}
	return g.group.GET(path, getGinHandlerFun(g.newContextFun, structTemplate, handlers...))
}

func (g *RouterGroup) Post(path string, desc string, handlers ...HandlerFunc) gin.IRoutes {
	return g.PostWithStructParams(path, desc, nil, handlers)
}
func (g *RouterGroup) PostWithStructParams(path string, desc string, structTemplate interface{}, handlers ...HandlerFunc) gin.IRoutes {
	g.Routes[path] = &RouteInfo{Desc: desc, Method: "POST", StructTemplate: structTemplate}
	return g.group.POST(path, getGinHandlerFun(g.newContextFun, structTemplate, handlers...))
}

func (g *RouterGroup) TravelGroupTree() map[string]*RouteInfo {
	m := make(map[string]*RouteInfo)
	for k, route := range g.Routes {
		if k[0] != '/' {
			k = "/" + k
		}
		m[k] = route
	}
	for k, subG := range g.GroupRoutes {
		gm := subG.TravelGroupTree()
		for k1, v1 := range gm {
			if k1[0] != '/' {
				k1 = "/" + k1
			}
			m[k+k1] = v1
		}
	}
	return m
}

type Engine struct {
	addr          string
	basePath      string
	ginEngine     *gin.Engine
	GroupRoutes   map[string]*RouterGroup // 组路由
	Routes        map[string]*RouteInfo   // 直接路由
	newContextFun func() Context
}

func NewEngine(addr string, newContextFun func() Context) *Engine {
	engine := &Engine{
		addr:          addr,
		ginEngine:     gin.Default(),
		newContextFun: newContextFun,
		GroupRoutes:   make(map[string]*RouterGroup),
		Routes:        make(map[string]*RouteInfo),
	}
	engine.ginEngine.SetTrustedProxies([]string{addr})
	return engine
}

func (e *Engine) EnableDebugMode() {
	gin.SetMode(gin.DebugMode)
}

func (e *Engine) Use(middleware ...HandlerFunc) {
	e.ginEngine.Use(getGinHandlerFun(e.newContextFun, nil, middleware...))
}

func (e *Engine) Group(path string, handlers ...HandlerFunc) *RouterGroup {
	grp := e.ginEngine.Group(path, getGinHandlerFun(e.newContextFun, nil, handlers...))
	grp1 := newRouterGroup(grp, e.newContextFun)
	e.GroupRoutes[path] = grp1
	return grp1
}

func (e *Engine) Get(path string, desc string, handlers ...HandlerFunc) gin.IRoutes {
	return e.GetWithStructParams(path, desc, nil, handlers...)
}

func (e *Engine) GetWithStructParams(path string, desc string, structTemplate interface{}, handlers ...HandlerFunc) gin.IRoutes {
	e.Routes[path] = &RouteInfo{Desc: desc, Method: "GET", StructTemplate: structTemplate}
	return e.ginEngine.GET(path, getGinHandlerFun(e.newContextFun, structTemplate, handlers...))
}

func (e *Engine) Post(path string, desc string, handlers ...HandlerFunc) gin.IRoutes {
	return e.PostWithStructParams(path, desc, nil, handlers...)
}

func (e *Engine) PostWithStructParams(path string, desc string, structTemplate interface{}, handlers ...HandlerFunc) gin.IRoutes {
	e.Routes[path] = &RouteInfo{Desc: desc, Method: "POST", StructTemplate: structTemplate}
	return e.ginEngine.POST(path, getGinHandlerFun(e.newContextFun, structTemplate, handlers...))
}

func (e *Engine) TravelGroupTree() map[string]*RouteInfo {
	m := make(map[string]*RouteInfo)
	for k, route := range e.Routes {
		if k[0] != '/' {
			k = "/" + k
		}
		m[k] = route
	}
	for k, subG := range e.GroupRoutes {
		gm := subG.TravelGroupTree()
		for k1, v1 := range gm {
			if k1[0] != '/' {
				k1 = "/" + k1
			}
			m[k+k1] = v1
		}
	}
	return m
}

func (e *Engine) Run() error {
	return e.ginEngine.Run(e.addr)
}

func (e *Engine) Stop() {

}

func (e *Engine) GetGinEngine() *gin.Engine {
	return e.ginEngine
}

func getGinHandlerFun(newContextFun func() Context, structTemplate interface{}, handlers ...HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := newContextFun()
		ctx.SetGinContext(c)
		for _, h := range handlers {
			if structTemplate != nil {
				receiver, field, value, err := structuredUnmarshaler(c, structTemplate)
				if err != nil {
					ctx.ResponseParseParamsFieldFail(c.FullPath(), field, value, err)
					c.Abort()
					return
				} else {
					reflect.ValueOf(h).Call([]reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(receiver)})
				}
			} else {
				reflect.ValueOf(h).Call([]reflect.Value{reflect.ValueOf(ctx)})
			}

			if c.IsAborted() {
				return
			}
		}
	}
}
