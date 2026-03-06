import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Analytics from './Analytics';
import type { AnalyticsResult } from '../types';

vi.mock('../api/products', () => ({
  getAnalytics: vi.fn(),
}));

vi.mock('./ProductImage', () => ({
  default: ({ name }: { name: string }) => <div data-testid="product-image">{name}</div>,
}));

import { getAnalytics } from '../api/products';

const mockAnalytics: AnalyticsResult = {
  mostPurchased: [
    { id: 'leche', name: 'LECHE ENTERA', purchaseCount: 5, currentPrice: 0.89 },
    { id: 'pan', name: 'PAN INTEGRAL', purchaseCount: 3, currentPrice: 1.25 },
  ],
  biggestIncreases: [
    { id: 'aceite', name: 'ACEITE OLIVA', firstPrice: 3.00, currentPrice: 6.00, increasePercent: 100.0 },
    { id: 'yogur', name: 'YOGUR NATURAL', firstPrice: 0.40, currentPrice: 0.60, increasePercent: 50.0 },
  ],
};

const emptyAnalytics: AnalyticsResult = {
  mostPurchased: [],
  biggestIncreases: [],
};

beforeEach(() => {
  vi.resetAllMocks();
});

describe('Analytics', () => {
  it('shows a loading indicator while data is being fetched', () => {
    vi.mocked(getAnalytics).mockReturnValue(new Promise(() => {}));
    render(<Analytics onSelectProduct={vi.fn()} />);
    expect(screen.getByRole('status', { hidden: true }) ?? document.querySelector('[aria-busy="true"]')).toBeTruthy();
  });

  it('renders both analytics sections with data', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('Productos más comprados')).toBeInTheDocument();
      expect(screen.getByText('Mayor subida de precio')).toBeInTheDocument();
    });
  });

  it('shows the most purchased products with name and count', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('LECHE ENTERA')).toBeInTheDocument();
      expect(screen.getByText('PAN INTEGRAL')).toBeInTheDocument();
    });

    expect(screen.getByText('5 veces')).toBeInTheDocument();
    expect(screen.getByText('3 veces')).toBeInTheDocument();
  });

  it('shows the products with the biggest price increase', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByText('ACEITE OLIVA')).toBeInTheDocument();
      expect(screen.getByText('YOGUR NATURAL')).toBeInTheDocument();
    });

    expect(screen.getByText('+100,0%')).toBeInTheDocument();
    expect(screen.getByText('+50,0%')).toBeInTheDocument();
  });

  it('shows empty message when there is no most-purchased data', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(emptyAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      const emptyMessages = screen.getAllByText('Aún no hay datos suficientes.');
      expect(emptyMessages).toHaveLength(2);
    });
  });

  it('shows an error message when the API fails', async () => {
    vi.mocked(getAnalytics).mockRejectedValue(new Error('Network error'));
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => {
      expect(screen.getByRole('alert')).toBeInTheDocument();
      expect(screen.getByRole('alert')).toHaveTextContent('Network error');
    });
  });

  it('calls onSelectProduct when clicking a most-purchased product', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    const onSelectProduct = vi.fn();
    render(<Analytics onSelectProduct={onSelectProduct} />);

    await waitFor(() => screen.getByText('LECHE ENTERA'));

    await userEvent.click(screen.getByText('LECHE ENTERA'));
    expect(onSelectProduct).toHaveBeenCalledWith('leche');
  });

  it('calls onSelectProduct when clicking a product with the biggest price increase', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    const onSelectProduct = vi.fn();
    render(<Analytics onSelectProduct={onSelectProduct} />);

    await waitFor(() => screen.getByText('ACEITE OLIVA'));

    await userEvent.click(screen.getByText('ACEITE OLIVA'));
    expect(onSelectProduct).toHaveBeenCalledWith('aceite');
  });

  it('shows the price range (first price → current price) for increases', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('ACEITE OLIVA'));

    // Prices must appear formatted in euros
    const priceTexts = screen.getAllByText(/3,00\s*€|6,00\s*€/);
    expect(priceTexts.length).toBeGreaterThanOrEqual(2);
  });

  it('shows the correct price range for yogurt', async () => {
    vi.mocked(getAnalytics).mockResolvedValue(mockAnalytics);
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('YOGUR NATURAL'));

    const priceTexts = screen.getAllByText(/0,40\s*€|0,60\s*€/);
    expect(priceTexts.length).toBeGreaterThanOrEqual(2);
  });

  it('shows "1 vez" in singular when purchaseCount is 1', async () => {
    vi.mocked(getAnalytics).mockResolvedValue({
      mostPurchased: [
        { id: 'sal', name: 'SAL FINA', purchaseCount: 1, currentPrice: 0.45 },
      ],
      biggestIncreases: [],
    });
    render(<Analytics onSelectProduct={vi.fn()} />);

    await waitFor(() => screen.getByText('SAL FINA'));
    expect(screen.getByText('1 vez')).toBeInTheDocument();
  });
});
