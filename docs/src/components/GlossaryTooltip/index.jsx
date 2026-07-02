import React, { useState } from "react";

import {
  autoUpdate,
  flip,
  FloatingFocusManager,
  FloatingPortal,
  offset,
  shift,
  useClick,
  useDismiss,
  useFloating,
  useInteractions,
  useRole,
} from "@floating-ui/react";
import { Icon } from "@iconify/react";
import ReactMarkdown from "react-markdown";

import glossary from "@generated/glossary-data/default/glossary.json";
import styles from "./styles.module.css";

export default function GlossaryTooltip({ term, children }) {
  const [isOpen, setIsOpen] = useState(false);

  const { refs, floatingStyles, context } = useFloating({
    open: isOpen,
    onOpenChange: setIsOpen,
    placement: "top",
    middleware: [offset(10), flip(), shift({ padding: 10 })],
    whileElementsMounted: autoUpdate,
  });

  const { getReferenceProps, getFloatingProps } = useInteractions([
    useClick(context),
    useDismiss(context),
    useRole(context),
  ]);

  if (!glossary[term]) {
    console.warn(`GlossaryTooltip: Term "${term}" not found`);
    return <span className={styles.notFound}>{children}</span>;
  }

  return (
    <>
      <span
        ref={refs.setReference}
        className={styles.term}
        {...getReferenceProps({
          "aria-label": `Show definition for ${glossary[term].title}`,
        })}
      >
        {children}
        <Icon icon="material-symbols:info-outline" width="12" height="12" className={styles.infoIcon} />
      </span>

      {isOpen && (
        <FloatingPortal>
          <FloatingFocusManager context={context} modal={false}>
            <div
              ref={refs.setFloating}
              className={styles.popover}
              style={floatingStyles}
              {...getFloatingProps()}
            >
              <strong className={styles.popoverTitle}>{glossary[term].title}</strong>
              <div className={styles.popoverContent}>
                <ReactMarkdown>{glossary[term].long}</ReactMarkdown>
              </div>
            </div>
          </FloatingFocusManager>
        </FloatingPortal>
      )}
    </>
  );
}
