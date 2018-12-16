package httpServer

var wsRoutes = WsRoutes{
	WsRoute{
		"hosts",
		"/api/v0/ws/Hosts",
		HandleWsHosts,
	},
}

var httpRoutes = HttpRoutes{
	HttpRoute{
		"HostsAll",
		"GET",
		"/api/v0/Hosts",
		HandleGetHosts,
	},
	HttpRoute{
		"ApiIndex",
		"GET",
		"/api{Path:.*}",
		HandleApiNotFound,
	},
}
