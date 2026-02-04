import React, { useContext, useRef } from "react";

import { Icon } from "@iconify/react";
import clsx from "clsx";

import ParamContext from "../ParamContext";
import styles from "./styles.module.css";

// InlineEditable is a parent component for an editable parameter in a
// ParamCodeBlock. InlineEditable can also be used directly to create a
// standalone, inline editable parameter.
const InlineEditable = ({ paramKey }) => {
  const { params, updateParam } = useContext(ParamContext);
  const inputRef = useRef(null);

  const handleChange = (e) => {
    updateParam(paramKey, e.target.value);
  };

  const handleIconClick = () => {
    if (inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  };

  return (
    <span className={styles.inlineEditable}>
      <input
        ref={inputRef}
        type="text"
        className={clsx("code-block-input", styles.input)}
        value={params[paramKey] || ""}
        onChange={handleChange}
        style={{
          width: `${Math.max((params[paramKey] || paramKey).length, paramKey.length)}ch`,
        }}
        aria-label={`Parameter ${paramKey}`}
      />
      <Icon
        icon="mdi:pencil"
        size={"48px"}
        onClick={handleIconClick}
        className={styles.icon}
        title="Edit parameter"
        aria-label={`Edit ${paramKey}`}
        role="button"
        tabIndex={0}
        onKeyPress={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            handleIconClick();
          }
        }}
      />
    </span>
  );
};

export default InlineEditable;
