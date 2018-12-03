GitBook Ace Plugin
===

This is a code editor plugin for GitBook, for inserting code segments into the book, with syntax highlight supports for about 110 types of programming languages.

See the plugin at work [here](http://ymcatar.gitbooks.io/gitbook-test/content/testing_ace.html).

## Changelog

* 0.3 Releases:
    * **0.3.2**: Fixed issue with code like `{{something}}` (angular expression)
    * **0.3.1**: Updated regular expression to better detect {%ace%} tags.
    * **0.3.0**: (Requires gitbook 3.0 or up) Improved support for gitbook 3.0+.


* 0.2 Releases:
    * **0.2.1**: Default to 'c_cpp' for pdf syntax highlight if language is not specified.
    * **0.2.0**: Added experimental support for pdf syntax highlight, please open issues for languages that are not working for you.


* 0.1 Releases:
    * **0.1.2**: Removed dark theme logic, fix theme error.
    * **0.1.1**: Added custom theme parameter.
    * **0.1.0**: Updated to latest version of ace (added Swift and JSX syntax support).


* 0.0 Releases:
    * **0.0.5**: Include pull requests from github, fixing scrolling.
    * **0.0.4**: Fixed syntax error in code.
    * **0.0.3**: Added option to disable syntax validation.
    * **0.0.2**: Added .pdf, .epub, .emobi format export support.
    * **0.0.1**: Initial release.

## Syntax

The plugin has a basic syntax:

```
{%ace edit=true, lang='c_cpp'%}
// This is a hello world program for C.
#include <stdio.h>

int main(){
  printf("Hello World!");
  return 1;
}
{%endace%}
```

* ```edit```: if this is set to true, the code will be editable by the user.

* ```lang```: the language for syntax highlight. If this is not specified, it will fallback to 'c_cpp' by default. For the full list of keyword for each language, please check out the github page of ace [here](https://github.com/ajaxorg/ace-builds/tree/master/src-min-noconflict), all the related files are starting with prefix ```mode-```. For instance:
    * mode_c_cpp.js ----> c_cpp
    * mode_java.js ----> java
    * ...

* ```check```: if this is set to false, syntax validation will be disabled.

* ```theme```: the theme for the editor, the default is 'chrome'.
    * monokai
    * coffee
    * ...
