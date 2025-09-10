<!-- markdownlint-disable MD041 -->
Using `contains` is commonly used to create validation messages
from performing a series of checks on an object. In this example, if
there are any `failures` then `allow` will be false. Using `contains`
makes it possible to build up `failures` incrementally.
