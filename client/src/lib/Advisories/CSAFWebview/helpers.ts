// This file is Free Software under the Apache-2.0 License
// without warranty, see README.md and LICENSES/Apache-2.0.txt for details.
//
// SPDX-License-Identifier: Apache-2.0
//
// SPDX-FileCopyrightText: 2023 German Federal Office for Information Security (BSI) <https://www.bsi.bund.de>
// Software-Engineering: 2023 Intevation GmbH <https://intevation.de>

export const innerLinkStyle = "hover:underline text-primary-700 dark:text-primary-400";

export const getReadableDateString: (
  datetime: string | undefined,
  intlFormat?: Intl.DateTimeFormat
) => string | undefined = (datetime: string | undefined, intlFormat?: Intl.DateTimeFormat) => {
  if (!datetime) {
    return datetime;
  }
  try {
    if (intlFormat) {
      const date = intlFormat.format(new Date(datetime));
      return date;
    }
    const date = new Date(datetime).toISOString(); // Ensure UTC by converting to Date and then back to ISO
    return date.replace(/\.[0]+Z$/, "Z").replace("T", " ");
  } catch (_e) {
    return datetime;
  }
};
