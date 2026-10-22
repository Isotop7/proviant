import fs from 'node:fs';

const edit = (file, pairs) => {
  let s = fs.readFileSync(file, 'utf8');
  const before = s;
  for (const [from, to] of pairs) {
    if (!s.includes(from)) {
      console.error(`NOT FOUND in ${file}: ${from.slice(0, 60)}`);
      process.exit(1);
    }
    s = s.split(from).join(to);
  }
  if (s === before) {
    console.error(`no change in ${file}`);
    process.exit(1);
  }
  fs.writeFileSync(file, s);
  console.log(`${file} updated`);
};

// The shell rename left the mobile rule on the old id.
edit('src/templates/scss/_productList.scss', [
  ['  #content-wrapper {', '  .app__content {'],
]);

// The shopping list's old single-underscore family is gone.
edit('src/templates/scss/_pages.scss', [
  [
    `.shopping-list-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 0;
  border-bottom: 1px solid var(--bs-border-color-translucent, var(--overlay-subtle));

  &:last-child {
    border-bottom: 0;
  }

  .item-name {
    font-weight: 500;
    color: var(--fg);
  }

  .item-notes {
    display: block;
    margin-top: 2px;
  }

  &.checked {
    opacity: .6;

    .item-name {
      color: var(--meta);
      text-decoration: line-through;
    }
  }
}

.shopping-list-item-notes-row {
  display: none;
  padding: 0 16px 8px 52px;

  &.expanded {
    display: block;
  }
}

`,
    '',
  ],
]);
