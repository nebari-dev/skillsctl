import { describe, it, expect } from 'vitest';
import { remark } from 'remark';
import remarkBaseLinks, { prefixUrl } from '../src/plugins/remark-base-links';

describe('prefixUrl', () => {
  const cases: Array<{ name: string; url: string; base: string; want: string }> = [
    { name: 'base "/" leaves links unchanged', url: '/cli/', base: '/', want: '/cli/' },
    { name: 'sub-path base prefixes internal links', url: '/cli/', base: '/skillsctl/', want: '/skillsctl/cli/' },
    { name: 'prefixes image paths', url: '/images/a.svg', base: '/skillsctl/', want: '/skillsctl/images/a.svg' },
    { name: 'never rewrites external links', url: 'https://example.com', base: '/skillsctl/', want: 'https://example.com' },
    { name: 'never rewrites protocol-relative links', url: '//example.com/x', base: '/skillsctl/', want: '//example.com/x' },
    { name: 'never rewrites anchor-only links', url: '#section', base: '/skillsctl/', want: '#section' },
    { name: 'preserves anchors on internal links', url: '/cli/#usage', base: '/skillsctl/', want: '/skillsctl/cli/#usage' },
    { name: 'idempotent on already-prefixed links', url: '/skillsctl/cli/', base: '/skillsctl/', want: '/skillsctl/cli/' },
  ];
  for (const c of cases) {
    it(c.name, () => {
      expect(prefixUrl(c.url, c.base)).toBe(c.want);
    });
  }
});

describe('remarkBaseLinks plugin', () => {
  it('rewrites link and image urls in a markdown document', async () => {
    const md = 'See [CLI](/cli/) and ![img](/images/a.svg) and [ext](https://x.io)';
    const out = String(
      await remark().use(remarkBaseLinks, { base: '/skillsctl/' }).process(md),
    );
    expect(out).toContain('(/skillsctl/cli/)');
    expect(out).toContain('(/skillsctl/images/a.svg)');
    expect(out).toContain('(https://x.io)');
  });

  it('is a no-op when base is "/"', async () => {
    const md = '[C](/cli/)';
    const out = String(await remark().use(remarkBaseLinks, { base: '/' }).process(md));
    expect(out).toContain('(/cli/)');
  });
});
