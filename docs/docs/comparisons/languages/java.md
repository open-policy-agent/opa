---
sidebar_label: Java
sidebar_position: 1
title: Java
---

<!-- markdownlint-disable MD044 -->

import SideBySideLanguageComparison from "@site/src/components/SideBySideLanguageComparison";

import { Ex1Title, Ex1Intro, Ex1Outro, Ex1Java, Ex1Rego } from './ex1.js';
import { Ex2Title, Ex2Intro, Ex2Java, Ex2Rego } from './ex2.js';
import { Ex3Title, Ex3Intro, Ex3Java, Ex3Rego } from './ex3.js';

# Java and Rego Language Comparison

Java is a general purpose programming language that is commonly used for
building web services, APIs and enterprise business applications.
Such applications often need to impose strict policies on the operations they perform
and the data they process. Java is also commonly used in domains that
are highly regulated, such as finance, healthcare, and government
where enforcing policies is even more critical.

These domains often come with complex policies to encode and enforce.
When doing so, Java code can become verbose and difficult to maintain.
This guide shows some examples of policy code in Java and the corresponding
Rego code for comparison.

<!-- markdownlint-disable MD044 -->

<SideBySideLanguageComparison
  title={Ex1Title}
  intro={Ex1Intro}
  outro={Ex1Outro}
  title1="Java"
  title2="Rego"
  lang1="javascript"
  lang2="rego"
  code1={Ex1Java}
  code2={Ex1Rego}
/>

<!-- markdownlint-disable MD044 -->

<SideBySideLanguageComparison
  title={Ex2Title}
  intro={Ex2Intro}
  title1="Java"
  title2="Rego"
  lang1="javascript"
  lang2="rego"
  code1={Ex2Java}
  code2={Ex2Rego}
/>

<!-- markdownlint-disable MD044 -->

<SideBySideLanguageComparison
  title={Ex3Title}
  intro={Ex3Intro}
  title1="Java"
  title2="Rego"
  lang1="javascript"
  lang2="rego"
  code1={Ex3Java}
  code2={Ex3Rego}
/>

<CardGrid>
  <Card key={"python"} item={{
    title: "Python",
    icon: require('./assets/images/python.png').default,
    link: "../languages/python",
    link_text: "Compare Python",
  }} />
  <Card key={"go"} item={{
    title: "Go",
    icon: require('./assets/images/go.png').default,
    link: "../languages/go",
    link_text: "Compare Go",
  }} />
</CardGrid>
