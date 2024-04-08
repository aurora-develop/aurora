const express = require("express");
const app = express();
const port = 3000;
const { createProxyMiddleware } = require("http-proxy-middleware");

app.use(
    "/",
    createProxyMiddleware({
        target: "http://127.0.0.1:8080/", // The request address that needs cross-domain processing
        changeOrigin: false, // Default is false, whether to change the original host header to the target URL
        ws: true,
        logLevel: "error"
    })
);

app.listen(port, () => console.log(`App listening on port ${port}!`));