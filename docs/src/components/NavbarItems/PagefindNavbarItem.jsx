import BrowserOnly from "@docusaurus/BrowserOnly";
import Head from "@docusaurus/Head";
import { Icon } from "@iconify/react";
import { useEffect } from "react";
import styles from "./PagefindNavbarItem.module.css";

function openPagefindModal() {
  document.querySelector("pagefind-modal")?.open();
}

export default function PagefindNavbarItem() {
  useEffect(() => {
    function onKeyDown(event) {
      const isMac = (navigator.userAgentData?.platform ?? navigator.platform).includes("Mac");
      const modKeyPressed = isMac ? event.metaKey : event.ctrlKey;
      if (modKeyPressed && event.key.toLowerCase() === "k") {
        event.preventDefault();
        openPagefindModal();
      }
    }
    document.addEventListener("keydown", onKeyDown);
    return () => document.removeEventListener("keydown", onKeyDown);
  }, []);

  return (
    <>
      <Head>
        <link href="/pagefind/pagefind-component-ui.css" rel="stylesheet" />
        <script src="/pagefind/pagefind-component-ui.js" type="module"></script>
      </Head>
      <div className={styles.item}>
        <button onClick={openPagefindModal} className={styles.searchButton} aria-label="Search">
          <Icon icon="mdi:magnify" className={styles.searchIcon} aria-hidden="true" />
          <span className={styles.searchText}>Search</span>
        </button>
      </div>
      <BrowserOnly>{() => <pagefind-modal></pagefind-modal>}</BrowserOnly>
    </>
  );
}
