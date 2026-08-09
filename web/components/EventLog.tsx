export default function EventLog({ lines }: { lines: string[] }) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 4,
        padding: 12,
        borderRadius: 10,
        border: "1px solid light-dark(#dcdcd8, #23272e)",
        background: "light-dark(#fff, #14171c)",
        maxHeight: 240,
        overflowY: "auto",
        fontSize: 11.5,
        lineHeight: 1.6,
      }}
    >
      {lines.length === 0 ? (
        <span style={{ color: "#7c828b" }}>(nothing has happened yet)</span>
      ) : (
        lines
          .slice()
          .reverse()
          .map((line, i) => (
            <div key={lines.length - i} style={{ color: "light-dark(#3a3f46, #c7ccd3)" }}>
              {line}
            </div>
          ))
      )}
    </div>
  );
}
