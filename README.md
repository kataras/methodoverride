# HTTP Method Override (Go)

[![build status](https://img.shields.io/github/workflow/status/kataras/methodoverride/CI/master?style=for-the-badge)](https://github.com/kataras/methodoverride/actions) [![report card](https://img.shields.io/badge/report%20card-a%2B-ff3333.svg?style=for-the-badge)](https://goreportcard.com/report/github.com/kataras/methodoverride) [![godocs](https://img.shields.io/badge/go-%20docs-488AC7.svg?style=for-the-badge)](https://pkg.go.dev/github.com/kataras/methodoverride)

The use of specific custom HTTP headers such as X-HTTP methods override can be very handy while developing and promoting a REST API. When deploying REST API based web services, you may encounter access limitations on both the server and client sides.

**Some Firewalls do not support PUT, DELETE or PATCH requests.**

The `methodoverride` package is a [net/http](https://pkg.go.dev/net/http) middleware. **It lets you use HTTP verbs such as PUT or DELETE in places where the client doesn't support it**.

## Getting started

The only requirement is the [Go Programming Language](https://go.dev/dl/).

```sh
$ go get github.com/kataras/methodoverride
```

```go
package main

import (
    "net/http"

    "github.com/kataras/methodoverride"
)

func main() {
    router := http.NewServeMux()

    mo := methodoverride.New( 
        // Defaults to nil. 
        // 
        methodoverride.SaveOriginalMethod("_originalMethod"), 
        // Default values. 
        // 
        // methodoverride.Methods(http.MethodPost), 
        // methodoverride.Headers("X-HTTP-Method",
        //                        "X-HTTP-Method-Override",
        //                        "X-Method-Override"), 
        // methodoverride.FormField("_method"), 
        // methodoverride.Query("_method"), 
    ) 

    router.HandleFunc("/path", func(w http.ResponseWriter, r *http.Request) {
        resp := "post response"

        if r.Method == http.MethodDelete {
            resp = "delete response"
        }

        w.Write([]byte(resp))
    })

    // Wrap your "router" with the methodoverride wrapper. 
    http.ListenAndServe(":8080", mo(router))
}

```

A **client** can request with POST, the server will respond like if it were a DELETE method.

```js
fetch("/path", {
    method: 'POST',
    headers: {
      "X-HTTP-Method": "DELETE"
    },
  })
  .then((resp)=>{
      // response body will be "delete response". 
 })).catch((err)=> { console.error(err) })
```

## License

Methodoverride is free and open-source software licensed under the [MIT License](https://tldrlegal.com/license/mit-license).
