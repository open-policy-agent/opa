const http = require('http')

const port     = 8000
const hostname = '0.0.0.0'

http.createServer((req, res) => res.end(`
Hello World, from the future!
It's ${
  new Date().getFullYear() + Math.ceil(Math.random() * 100)
    } here, how is it going back there in ${new Date().getFullYear()}? :)

`))
  .listen(port, hostname, () => console.info(`listening on http://${hostname}:${port}`))
