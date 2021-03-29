#!/usr/bin/env python3
from time import sleep

from markdown import markdown
from bs4 import BeautifulSoup
import sys
from requests import get
from os.path import abspath, dirname, exists, isfile
import re
import validators


class bcolors:
    HEADER = '\033[95m'
    OKGREEN = '\033[92m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'


def fail(item):
    print(bcolors.FAIL + "{} is broken!".format(item) + bcolors.ENDC)


def ok(item):
    print(bcolors.OKGREEN + "{} is valid".format(item) + bcolors.ENDC)


def match_header(link, doc) -> int:
    anchors = re.findall("^#+(.*)", link)
    if len(anchors) > 0:
        a = anchors[0].strip().lower()
        if a.replace(" ", "-") in \
                [a.text.replace(" ", "-").lower().strip() for a in doc.find_all(re.compile('^h[1-6]$'))]:
            return True
        else:
            return False


if __name__ == '__main__':
    if len(sys.argv) > 0:
        file = sys.argv[1]
    else:
        print("require file as argument to parse")
        exit()

    if ".md" not in file:
        print("{} not a file or not a markdown. Skip".format(file))
        exit()

    result = 0
    try:
        with open(file, "r") as md:
            print(bcolors.HEADER + "Parsing {}...".format(file) + bcolors.ENDC)
            doc = BeautifulSoup(markdown(md.read()), 'html.parser')
            for link in doc.find_all('a'):
                if link.get('href') is None:
                    continue
                if link.get('href').startswith("#"):
                    if match_header(link.get("href"), doc):
                        ok(link.get("href"))
                    else:
                        fail(link.get("href"))
                        result += 1
                else:
                    url = link.get('href')
                if validators.url(url):
                    resp = get(url)
                    # we dont want to ddos anybody
                    # github could return 429 if too many requests
                    if "github" in url:
                        sleep(1)
                    if resp.status_code != 200:
                        result += 1
                        print(bcolors.FAIL + "{} - Returned {}".format(url, resp.status_code) + bcolors.ENDC)
                    else:
                        print(bcolors.OKGREEN + "{} - Returned {}".format(url, resp.status_code) + bcolors.ENDC)
                else:
                    if "#" in url:
                        target_file, anchor = re.findall(r'(.*?)#+(\w.*)', url)[0]
                    else:
                        anchor = ""
                        target_file = url
                    status = exists(abspath(dirname(file) + "/" + target_file))
                    if status:
                        if anchor:
                            if isfile(abspath(dirname(file) + "/" + target_file)):
                                with open(abspath(dirname(file) + "/" + target_file), "r") as testfile:
                                    doc = BeautifulSoup(markdown(testfile.read()), 'html.parser')
                                    if match_header("#" + anchor, doc):
                                        ok(url)
                                    else:
                                        fail(url)
                                        anchor = ""
                                        result += 1
                                        continue
                            else:
                                result += 1
                                fail(url)
                                continue
                        else:
                            ok(url)

                    else:
                        result += 1
                        fail(url)

    except Exception as e:
        print("Fail while running {}\nError: {}".format(sys.argv[1], e))

    sys.exit(result)
