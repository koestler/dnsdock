package httpServer

var wsRoutes = WsRoutes{
}

var httpRoutes = HttpRoutes{
	HttpRoute{
		"HostsAll",
		"GET",
		"/api/v0/Hosts",
		HandleHostsGet,
	},
	HttpRoute{
		"ApiIndex",
		"GET",
		"/api{Path:.*}",
		HandleApiNotFound,
	},
}
