rpc
===
[![Build Status](https://travis-ci.org/gorilla/rpc.png?branch=master)](https://travis-ci.org/gorilla/rpc)

gorilla/rpc is a foundation for RPC over HTTP services, providing access to the exported methods of an object through HTTP requests.

Read the full documentation here: https://www.gorillatoolkit.org/pkg/rpc

## Fork ##
Fork specific changes not yet merged to source (github.com/gorilla/rpc)

#### 1) Allow RPC methods to set response headers

Support an alternate method signature that includes the http.Header instance for the response. This allows RPC methods to set any relevant headers, such as setting a cookie through the Set-Cookie header.
