# Site Updates

We use GitHub pages to host the website that includes all of the OPA documentation. In order to update the website, you need to have write permission on the open-policy-agent/opa repository.

You also need to have [gulp](http://gulpjs.com/) installed to build the site.

## Run Site Locally

1. For first time user, run

    ```
    npm install
    ```

2. To start the server, run

    ```
    gulp
    ```

## Run Documentation Locally

    1. For first time user, run

        ```
        gitbook install
        ```

    2. To start the server, run

        ```
        gitbook serve
        ```

    3. Once you are happy about your changes to documentation, run

        ```
        gitbook build
        cd ..
        gulp doc-build
        ```

## Build site

Once you are happy about your changes to the site and wanna deploy it, run

    ```
    gulp build
    ```
And you will find a folder called "deploy" being genarated. Deploy files inside to gh-pages.
