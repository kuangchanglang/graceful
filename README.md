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

# Drawback
```graceful``` starts a master process to keep pid unchaged for process managers(systemd, supervisor, etc.), and a worker proccess listen to actual addrs. That means ```graceful``` starts one more process. Fortunately, master proccess waits for signals and reload worker when neccessary, which is costless since reload is usually low-frequency action. 

# TODO
- ListenAndServeTLS
- Run in only one process without master-worker
