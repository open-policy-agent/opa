# Changelog

## 2.2.0 - Nov 23, 2013
* fix simple or queries
* convert respond-to to use sass maps
* convert context to use sass maps

## 2.0.7 - Sept 17th, 2013
* fix fallback support for 1.x

## 2.0.0 - The Past
* Looks like we forgot relase notes for 2.0. oops

## 1.3 - August 28th, 2012
* better conversion to base-ems
* fix floating point error

## 1.2 - August 16th, 2012
* Added ability to force the 'all' media type to be written by setting `$breakpoint-force-media-all: true;`. Defaults to `false`.
* Added ability to generate no query fallback code. See the README for full documentaiton.

## 1.1.1 - July 30, 2012
* Added (forgot to include the first time) the ability to query the media type using `breakpoint-get-context('media')`


## 1.1 - July 29, 2012
* Added function `breakpoint-get-context($feature)` to allow users to get the current media query context

## 1.0.2 - July 28, 2012
* Refixed our 'device-pixel-ratio' conversions because, frankly, the w3c was wrong.
* fixed bugs that caused single and triple value single queries to fail. Also bugs with stacking single and triple value queries.

## 1.0.1 - June 27, 2012
* fixed logic error that would print multiple instences of a media type

## 1.0 - June 22, 2012
* Refactor of the underlying logic to make everything work better and make the world a happy place.
* Added default options for Default Feature, Default Media, and Default Feature Pair.
* Changed default media from "Screen" to "All".
* Added ability to have all px/pt/percentage media queries transformed into em based media queries.

## 0.3 - June 18, 2012
* Rewrote 'device-pixel-ratio' conversions to change from prefixed nightmarish hell to Resolution standard based on the [W3C Unprefixing -webkit-device-pixel-ratio article](http://www.w3.org/blog/CSS/2012/06/14/unprefix-webkit-device-pixel-ratio/)
* Large README update covering feature set, installation, assumptions, and more.

## 0.2 - May 24, 2012
* Converted from Sass to SCSS
* Converted README examples from Sass to SCSS
* Added ability to do min/max easily with any valid feature
* Added prefixing for "device-pixel-ratio" feature for the three implementations (-webkit, -moz, -o) as well as a standard version for future friendliness
  * -moz's min/max is different than -webkit or -o, so prefixed differently
  * Opera is strange and needs its DPR in a ratio instead of a floating point number, so requires the fraction rubygem and has a numerator/denominator function to accommodate.
* Added ability to have single feature/value input be either have feature first or second

## 0.1 - May 22, 2012
* extract breakpoint from survival kit to this gem
