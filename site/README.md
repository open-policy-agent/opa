# Site Updates

We use GitHub pages to host the website that includes all of the OPA documentation. In order to update the website, you need to have write permission on the open-policy-agent/opa repository.

You also need to have [Jekyll](http://jekyllrb.com) installed to build the site. If you are not sure how to install Jekyll, see the link for details.

Assuming you have Ruby installed, all you should need to do is run:

```
gem install --user-install jekyll
gem install --user-install autoprefixer-rails
gem install --user-install jekyll-assets
gem install --user-install jekyll-contentblocks
gem install --user-install jekyll-minifier
```

To update the website perform the following steps:

1. Obtain a fresh copy of the repository

    ```
    git clone git@github.com:open-policy-agent/opa.git opa-site
    ```

    - Note: if you are preparing documentation for a specific release, checkout the release tag in this step as well.

1. Build the site content and save the output:

    ```
    cd opa-site/site
    jekyll build .
    tar czvf ~/site.tar.gz -C _site .
    ```

1. Checkout the gh-pages branch and overlay the new site content:

    ```
    git checkout gh-pages
    tar zxvf ~/site.tar.gz
    git commit -a -m "Updating site for release 0.12.8"
    ```

1. Push the gh-pages branch back to GitHub:

    ```
    git push origin gh-pages
    ```

## REST API Examples

The REST API specification contains examples that are generated manually by running
`./_scripts/rest-examples/gen-examples.sh`. This script will launch OPA and
execute a series of API calls to produce output can be copied into the specification.
