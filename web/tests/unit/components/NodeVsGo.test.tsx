import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { NodeVsGo } from '../../../src/components/NodeVsGo';
import { MIGRATE } from '../../../src/data';

describe('NodeVsGo', () => {
  it('renders the comparison heading and both Ruby and Go columns', () => {
    const { container } = render(<NodeVsGo lib={MIGRATE} />);
    expect(container.querySelector(`#${MIGRATE.id}-cmp`)).not.toBeNull();
    expect(screen.getByText('Ruby')).toBeInTheDocument();
    expect(screen.getByText('Go')).toBeInTheDocument();
    expect(container.querySelectorAll('.compare .code').length).toBe(2);
  });
});
