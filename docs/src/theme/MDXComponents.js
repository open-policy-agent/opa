import MDXComponents from "@theme-original/MDXComponents";
import TabItem from "@theme-original/TabItem";
import Tabs from "@theme-original/Tabs";

import BuiltinTable from "@site/src/components/BuiltinTable";
import Card from "@site/src/components/Card";
import CardGrid from "@site/src/components/CardGrid";
import EcosystemEmbed from "@site/src/components/EcosystemEmbed";
import EcosystemFeatureLink from "@site/src/components/EcosystemFeatureLink";
import EvergreenCodeBlock from "@site/src/components/EvergreenCodeBlock";
import PlaygroundExample from "@site/src/components/PlaygroundExample";
import RunSnippet from "@site/src/components/RunSnippet";
import SideBySideColumn from "@site/src/components/SideBySide/Column";
import SideBySideContainer from "@site/src/components/SideBySide/Container";

export default {
  ...MDXComponents,
  TabItem,
  Tabs,

  // custom components with broad use
  BuiltinTable,
  Card,
  CardGrid,
  EcosystemEmbed,
  EcosystemFeatureLink,
  EvergreenCodeBlock,
  PlaygroundExample,
  RunSnippet,
  SideBySideColumn,
  SideBySideContainer,
};
