import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { DocsView } from '../../../src/components/DocsView';
import type { DocIndex } from 'go-ui';

// A minimal DocIndex the stubbed fetch returns for DocsApp's doc.json request.
const DOC_INDEX: DocIndex = {
  module: 'github.com/malcolmston/migrate',
  packages: [
    {
      importPath: 'github.com/malcolmston/migrate',
      name: 'migrate',
      synopsis: 'Package migrate is a standard-library-only, ActiveRecord-style schema migration toolkit.',
      doc: 'Package migrate is a standard-library-only, ActiveRecord-style schema migration toolkit.',
      consts: [],
      vars: [],
      types: [
        {
          name: 'Migrator',
          signature: 'type Migrator struct{}',
          doc: 'Migrator wraps a *sql.DB and drives migrations forward and backward.',
          consts: [],
          vars: [],
          funcs: [],
          methods: [],
        },
      ],
      funcs: [{ name: 'New', signature: 'func New(db *sql.DB, opts ...Option) *Migrator', doc: 'New constructs a Migrator over db.' }],
    },
  ],
};

describe('DocsView', () => {
  beforeEach(() => {
    // DocsApp fetches doc.json; return the small index.
    global.fetch = vi.fn((input: RequestInfo | URL) => {
      if (String(input).includes('doc.json')) {
        return Promise.resolve({ ok: true, json: () => Promise.resolve(DOC_INDEX) } as Response);
      }
      return new Promise<Response>(() => {});
    }) as unknown as typeof fetch;
  });

  it('renders the inline React API reference from the fetched doc.json', async () => {
    const { container } = render(<DocsView />);
    expect(container.querySelector('#view-docs')).not.toBeNull();
    expect(
      screen.getByRole('heading', { level: 2, name: /API documentation/ }),
    ).toBeInTheDocument();

    // DocsApp fetches asynchronously, then renders the package view + symbols.
    expect(await screen.findByRole('heading', { name: /package migrate/ })).toBeInTheDocument();
    expect(container.querySelector('#sym-New'), 'func New symbol card').not.toBeNull();
    expect(container.querySelector('#sym-Migrator'), 'type Migrator symbol card').not.toBeNull();

    // The secondary link to the raw generated static HTML remains.
    expect(screen.getByRole('link', { name: /Open the raw generated HTML/ })).toHaveAttribute('href', './api/');
  });
});
