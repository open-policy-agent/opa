---
sidebar_label: Python
sidebar_position: 2
title: Python
---

<!-- markdownlint-disable MD044 -->

import SideBySideLanguageComparison from "@site/src/components/SideBySideLanguageComparison";

import { Ex1Title, Ex1Intro, Ex1Outro, Ex1Python, Ex1Rego } from './ex1.js';
import { Ex2Title, Ex2Intro, Ex2Python, Ex2Rego } from './ex2.js';
import { Ex3Title, Ex3Intro, Ex3Python, Ex3Rego } from './ex3.js';

# Python and Rego Language Comparison

Python is a versatile programming language commonly used in domains such as
data analysis, web development, automation, and scientific computing.
Python applications frequently interact with external data sources,
handle sensitive information,
and process unstructured and untrusted data.

Being so general-purpose Python can do all of these things, but
expressing the related policies and rules can be error-prone and
distract from the core logic of applications.
This guide presents a series of examples illustrating how a policy
can be expressed in Python and the corresponding Rego code for comparison.

<SideBySideLanguageComparison
  title={Ex1Title}
  intro={Ex1Intro}
  outro={Ex1Outro}
  title1="Python"
  title2="Rego"
  lang1="python"
  lang2="rego"
  code1={Ex1Python}
  code2={Ex1Rego}
/>

<SideBySideLanguageComparison
  title={Ex2Title}
  intro={Ex2Intro}
  title1="Python"
  title2="Rego"
  lang1="python"
  lang2="rego"
  code1={Ex2Python}
  code2={Ex2Rego}
/>

<SideBySideLanguageComparison
  title={Ex3Title}
  intro={Ex3Intro}
  title1="Python"
  title2="Rego"
  lang1="python"
  lang2="rego"
  code1={Ex3Python}
  code2={Ex3Rego}
/>

<CardGrid>
  <Card key={"java"} item={{
    title: "Java",
    icon: require('./assets/images/java.png').default,
    link: "../languages/java",
    link_text: "Compare Java",
  }} />
  <Card key={"go"} item={{
    title: "Go",
    icon: require('./assets/images/go.png').default,
    link: "../languages/go",
    link_text: "Compare Go",
  }} />
</CardGrid>
