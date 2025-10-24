package traefik

type router struct {
	EntryPoints []string `json:"entryPoints"`
	Middlewares []string `json:"middlewares"`
	Service     string   `json:"service"`
	Rule        string   `json:"rule"`
	Priority    int      `json:"priority"`
	TLS         struct {
		Options      string `json:"options"`
		CertResolver string `json:"certResolver"`
	} `json:"tls"`
	Observability struct {
		AccessLogs     bool   `json:"accessLogs"`
		Metrics        bool   `json:"metrics"`
		Tracing        bool   `json:"tracing"`
		TraceVerbosity string `json:"traceVerbosity"`
	} `json:"observability"`
	Status   string   `json:"status"`
	Using    []string `json:"using"`
	Name     string   `json:"name"`
	Provider string   `json:"provider"`
}
type entrypoint struct {
	Address   string `json:"address"`
	Transport struct {
		LifeCycle struct {
			GraceTimeOut string `json:"graceTimeOut"`
		} `json:"lifeCycle"`
		RespondingTimeouts struct {
			ReadTimeout string `json:"readTimeout"`
			IdleTimeout string `json:"idleTimeout"`
		} `json:"respondingTimeouts"`
	} `json:"transport"`
	ForwardedHeaders struct {
	} `json:"forwardedHeaders"`
	HTTP struct {
		SanitizePath   bool `json:"sanitizePath"`
		MaxHeaderBytes int  `json:"maxHeaderBytes"`
	} `json:"http"`
	HTTP2 struct {
		MaxConcurrentStreams int `json:"maxConcurrentStreams"`
	} `json:"http2"`
	UDP struct {
		Timeout string `json:"timeout"`
	} `json:"udp"`
	Name string `json:"name"`
}
