"use client";

import { useEffect, useState } from "react";

function parseTimestamp(value: string): Date | null {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatTwinTimestamp(value: string): string {
  return formatTwinTimestampFor(value);
}

function formatTwinTimestampFor(value: string, locale?: string, timeZone?: string): string {
  const date = parseTimestamp(value);
  if (!date) return value;

  return new Intl.DateTimeFormat(locale, {
    dateStyle: "medium",
    timeStyle: "short",
    timeZone,
  }).format(date);
}

export function TwinTimestamp({ value }: { value: string }) {
  const isTimestamp = parseTimestamp(value) !== null;
  const [formatted, setFormatted] = useState(() => formatTwinTimestampFor(value, "en-US", "UTC"));

  useEffect(() => {
    if (isTimestamp) setFormatted(formatTwinTimestamp(value));
  }, [isTimestamp, value]);

  if (!isTimestamp) return <span>{value}</span>;
  return <time dateTime={value} suppressHydrationWarning>{formatted}</time>;
}
