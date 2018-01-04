# graceful
Inspired by [overseer](https://github.com/fvbock/endless) and [endless](https://github.com/fvbock/endless), with minimum codes and handy api to make http server graceful.

# Prerequisite
golang 1.8+

# Feature
- Graceful reload http servers, zero downtime on upgrade.
- Compatible with systemd, supervisor, etc.
- Drop-in placement for ```http.ListenAndServe```

# Example 
``` go 
    type handler struct {
    }

    func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
        fmt.Fprintf(w, "Hello, port: %v, %q", r.Host, html.EscapeString(r.URL.Path))
    }

    func main(){
	    graceful.ListenAndServe(":9222", &handler{})
    }
```

multi servers:
```go
    func main(){
        server := graceful.NewServer()
        server.Register("0.0.0.0:9223", &handler{})
        server.Register("0.0.0.0:9224", &handler{})
        server.Register("0.0.0.0:9225", &handler{})
        err := server.Run()
        fmt.Printf("error: %v\n", err)
    }
```

More example checkout example folder.

# TODO
- ListenAndServeTLS
- Run in only one process without master-worker
