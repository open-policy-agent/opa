---
sidebar_label: Go
sidebar_position: 4
title: Go
---

<!-- markdownlint-disable MD044 -->

import SideBySideLanguageComparison from "@site/src/components/SideBySideLanguageComparison";

import { Ex1Title, Ex1Intro, Ex1Outro, Ex1Go, Ex1Rego } from './ex1.js';
import { Ex2Title, Ex2Intro, Ex2Go, Ex2Rego } from './ex2.js';
import { Ex3Title, Ex3Intro, Ex3Go, Ex3Rego } from './ex3.js';

# Go and Rego Language Comparison

Go is a statically typed, compiled programming language.
It is commonly used in cloud native environments
for building services and APIs.

Go is generally well suited to such use cases.
However, when it comes to expressing policy code,
it can be verbose when expressing large policies that must unpack unstructured or untrusted data safely.

This guide presents a series of examples illustrating how a policy can be expressed in Go,
and the corresponding Rego code for comparison.

<SideBySideLanguageComparison
  title={Ex1Title}
  intro={Ex1Intro}
  outro={Ex1Outro}
  title1="Go"
  title2="Rego"
  lang1="go"
  lang2="rego"
  code1={Ex1Go}
  code2={Ex1Rego}
/>

<SideBySideLanguageComparison
  title={Ex2Title}
  intro={Ex2Intro}
  title1="Go"
  title2="Rego"
  lang1="go"
  lang2="rego"
  code1={Ex2Go}
  code2={Ex2Rego}
/>

<SideBySideLanguageComparison
  title={Ex3Title}
  intro={Ex3Intro}
  title1="Go"
  title2="Rego"
  lang1="go"
  lang2="rego"
  code1={Ex3Go}
  code2={Ex3Rego}
/>

<CardGrid>
  <Card key={"python"} item={{
    title: "Python",
    icon: require('./assets/images/python.png').default,
    link: "../languages/python",
    link_text: "Compare Python",
  }} />
  <Card key={"java"} item={{
    title: "Java",
    icon: require('./assets/images/java.png').default,
    link: "../languages/java",
    link_text: "Compare Java",
  }} />
</CardGrid>
