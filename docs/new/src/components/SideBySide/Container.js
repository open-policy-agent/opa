import React from "react";

export default function SideBySideContainer({ children }) {
  return (
    <div className={`side-by-side`}>
      {children}
    </div>
  );
}
