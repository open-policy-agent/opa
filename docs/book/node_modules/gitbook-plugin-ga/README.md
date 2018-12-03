Google Analytics tracking for GitBook
==============

You can use install it via **NPM**:

```
$ npm install gitbook-plugin-ga
```

And use it for your book with in the book.json:

```
{
    "plugins": ["ga"]
}
```

You can set the Google Analytics tracking ID using the plugins configuration in the book.json:

```
{
    "plugins": ["ga"],
    "pluginsConfig": {
        "ga": {
            "token": "UA-XXXX-Y"
        }
    }
}
```

You can customize the tracker object by passing additional configuration options. You can either pass in `auto`, `none` or an object:

```
{
    "plugins": ["ga"],
    "pluginsConfig": {
        "ga": {
            "token": "UA-XXXX-Y",
            "configuration": {
                "cookieName": "new_cookie_name",
                "cookieDomain": "mynew.domain.com"
            }
        }
    }
}
```

For an overview of all available configuration parameters, please refer to the [analytics.js field reference](https://developers.google.com/analytics/devguides/collection/analyticsjs/field-reference#create).
