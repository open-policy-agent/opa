#!/usr/bin/env python3
import signal

from time import sleep

from markdown import markdown
from bs4 import BeautifulSoup
import sys
from requests import get
from os.path import abspath, dirname, exists, isfile
import re
import validators
import argparse
import yaml

config = 'config.yml'
regex = False
result = 0


class bcolors:
    HEADER = '\033[95m'
    OKGREEN = '\033[92m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'


def int_handler(sig, frame):
    print('Exit via interrupt')
    sys.exit(result)


def fail(item):
    print(bcolors.FAIL + "{} is broken!".format(item) + bcolors.ENDC)


def ok(item):
    print(bcolors.OKGREEN + "{} is valid".format(item) + bcolors.ENDC)


def match_header(lnk, document) -> int:
    anchors = re.findall("^#+(.*)", lnk)
    if len(anchors) > 0:
        a = anchors[0].strip().lower()
        if a.replace(" ", "-") in \
                [a.text.replace(" ", "-").lower().strip() for a in document.find_all(re.compile('^h[1-6]$'))]:
            return True
        else:
            return False


if __name__ == '__main__':
    signal.signal(signal.SIGINT, int_handler)
    parser = argparse.ArgumentParser(description='links checker.')
    parser.add_argument('-f', '--file', dest='file', required=True,
                        type=str, help='target file to parse')
    parser.add_argument('-v', '--verbose', dest='verbose', action='store_true',
                        help='verbosity (default False)')
    args = parser.parse_args()

    if ".md" not in args.file:
        print("{} not a file or not a markdown. Skip".format(args.file))
        exit()
    if isfile(config):
        regex_template = []
        with open(config, 'r') as template:
            exclusions = yaml.load(template, Loader=yaml.FullLoader)

            for item in exclusions['links_exclusions']:
                try:
                    re.compile(item.strip())
                    regex_template.append(item.strip())
                except re.error:
                    print("failed to parse line {} in {}".format(item.strip(), config))
        if len(regex_template) > 0:
            regex_template = re.compile("|".join(regex_template))
            regex = True

        if any(substring in args.file for substring in exclusions['path_exclusions']):
            print('skipping file {}'.format(args.file))
            exit(0)
    try:
        with open(args.file, "r") as md:
            print(bcolors.HEADER + "Parsing {}...".format(args.file) + bcolors.ENDC)
            doc = BeautifulSoup(markdown(md.read()), 'html.parser')
            for link in doc.find_all('a'):
                if link.get('href') is None:
                    continue
                if link.get('href').startswith("#"):
                    if match_header(link.get("href"), doc):
                        if args.verbose:
                            ok(link.get("href"))
                        continue
                    else:
                        fail(link.get("href"))
                        result += 1
                        continue
                else:
                    url = link.get('href')
                    if regex:
                        if regex_template.match(url):
                            if args.verbose:
                                print('{} matched regexp, skipping...'.format(url))
                            continue
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
                        if args.verbose:
                            print(bcolors.OKGREEN + "{} - Returned {}".format(url, resp.status_code) + bcolors.ENDC)
                else:
                    if "#" in url:
                        target_file, anchor = re.findall(r'(.*?)#+(\w.*)', url)[0]
                    else:
                        anchor = ""
                        target_file = url
                    status = exists(abspath(dirname(args.file) + "/" + target_file))
                    if status:
                        if anchor:
                            if isfile(abspath(dirname(args.file) + "/" + target_file)):
                                with open(abspath(dirname(args.file) + "/" + target_file), "r") as testfile:
                                    doc = BeautifulSoup(markdown(testfile.read()), 'html.parser')
                                    if match_header("#" + anchor, doc):
                                        if args.verbose:
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
                            if args.verbose:
                                ok(url)

                    else:
                        result += 1
                        fail(url)

    except Exception as e:
        print("Fail while running {}\nError: {}".format(args.file, e))

    sys.exit(result)
