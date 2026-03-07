import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import ProductDetail from './ProductDetail';
import * as productsApi from '../api/products';
import type { Product } from '../types';

vi.mock('../api/products', () => ({
  getProduct: vi.fn(),
  updateProductImage: vi.fn(),
  deletePriceRecord: vi.fn(),
  getAccumulatedIPC: vi.fn(),
}));
vi.mock('recharts', () => ({
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  LineChart: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  Line: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  Tooltip: () => null,
}));

const mockProduct: Product = {
  id: '1',
  name: 'LECHE ENTERA HACENDADO 1L',
  category: 'Lácteos',
  currentPrice: 0.89,
  priceHistory: [
    { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona' },
    { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona' },
  ],
};

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(productsApi.getAccumulatedIPC).mockResolvedValue({
    from_year: 2025,
    to_year: 2025,
    accumulated_rate: 0.025,
  });
});

describe('ProductDetail', () => {
  it('shows "Cargando producto..." while loading', () => {
    vi.mocked(productsApi.getProduct).mockReturnValue(new Promise(() => {}));
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    expect(screen.getByText('Cargando producto...')).toBeInTheDocument();
  });

  it('shows the product name after loading', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText('LECHE ENTERA HACENDADO 1L')).toBeInTheDocument()
    );
  });

  it('shows the product category', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Lácteos')).toBeInTheDocument());
  });

  it('shows the current price formatted in the header', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => {
      const priceEl = document.querySelector('.detail-header .price');
      expect(priceEl).toHaveTextContent('0,89 €');
    });
  });

  it('shows the price history table', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('Evolución del precio'));
    expect(screen.getByText('Fecha')).toBeInTheDocument();
    expect(screen.getByText('Precio')).toBeInTheDocument();
    expect(screen.getByText('Tienda')).toBeInTheDocument();
  });

  it('shows the stores in the table', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('Evolución del precio'));
    expect(screen.getAllByText('Mercadona')).toHaveLength(2);
  });

  it('shows a not found message when getProduct fails', async () => {
    vi.mocked(productsApi.getProduct).mockRejectedValue(new Error('Not found'));
    render(<ProductDetail productId="9999" onBack={vi.fn()} />);
    await waitFor(() =>
      expect(screen.getByText('Producto no encontrado')).toBeInTheDocument()
    );
  });

  it('calls onBack when the back button is pressed', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    const onBack = vi.fn();
    render(<ProductDetail productId="1" onBack={onBack} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await userEvent.click(screen.getByRole('button', { name: /volver/i }));
    expect(onBack).toHaveBeenCalledOnce();
  });

  it('calls getProduct with the correct ID', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    expect(productsApi.getProduct).toHaveBeenCalledWith('1');
  });

  it('shows a price-change badge when there are at least 2 price records', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    // 0.79 → 0.89 = +12.7%
    expect(screen.getByText('+12,7%')).toBeInTheDocument();
  });

  it('shows the badge in red (--up modifier) when price increased', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('+12,7%'));
    const badge = screen.getByText('+12,7%').closest('.price-change-badge');
    expect(badge).toHaveClass('price-change-badge--up');
  });

  it('shows the badge in green (--down modifier) when price decreased', async () => {
    const cheaperProduct: Product = {
      ...mockProduct,
      currentPrice: 0.69,
      priceHistory: [
        { date: '2025-01-15T00:00:00Z', price: 0.89, store: 'Mercadona' },
        { date: '2025-09-22T00:00:00Z', price: 0.69, store: 'Mercadona' },
      ],
    };
    vi.mocked(productsApi.getProduct).mockResolvedValue(cheaperProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    // 0.89 → 0.69 = -22.5%
    const badge = screen.getByText('-22,5%').closest('.price-change-badge');
    expect(badge).toHaveClass('price-change-badge--down');
  });

  it('shows the accumulated IPC when the API returns data', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    vi.mocked(productsApi.getAccumulatedIPC).mockResolvedValue({
      from_year: 2025,
      to_year: 2025,
      accumulated_rate: 0.025,
    });
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    await waitFor(() =>
      expect(document.querySelector('.detail-header__ipc-value')).toHaveTextContent('+2,5%'),
    );
  });

  it('does not show the IPC block when accumulated_rate is 0', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    vi.mocked(productsApi.getAccumulatedIPC).mockResolvedValue({
      from_year: 2025,
      to_year: 2025,
      accumulated_rate: 0,
    });
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    expect(document.querySelector('.detail-header__ipc')).not.toBeInTheDocument();
  });

  it('shows the max price with date when there are at least 2 price records', async () => {
    vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    // max is 0.89 (the second record)
    expect(document.querySelector('.detail-header__max-price-value')).toHaveTextContent('0,89 €');
  });

  it('does not show the max price block when there is only one price record', async () => {
    const singleRecordProduct: Product = {
      ...mockProduct,
      priceHistory: [{ date: '2025-01-15T00:00:00Z', price: 0.89, store: 'Mercadona' }],
    };
    vi.mocked(productsApi.getProduct).mockResolvedValue(singleRecordProduct);
    render(<ProductDetail productId="1" onBack={vi.fn()} />);
    await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
    expect(document.querySelector('.price-change-badge')).not.toBeInTheDocument();
    expect(document.querySelector('.detail-header__max-price')).not.toBeInTheDocument();
  });

  describe('ImageEditor', () => {
    it('shows the edit image button after loading', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      expect(screen.getByRole('button', { name: 'Cambiar imagen del producto' })).toBeInTheDocument();
    });

    it('shows URL input when the edit button is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      expect(screen.getByLabelText('URL de imagen del producto')).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Guardar imagen' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument();
    });

    it('hides the input when cancel is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cambiar imagen del producto' })).toBeInTheDocument();
    });

    it('calls updateProductImage and closes input on success', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      vi.mocked(productsApi.updateProductImage).mockResolvedValue('https://prod.mercadona.com/img/leche.jpg');
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      fireEvent.change(screen.getByLabelText('URL de imagen del producto'), {
        target: { value: 'https://prod.mercadona.com/img/leche.jpg' },
      });
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      await waitFor(() =>
        expect(productsApi.updateProductImage).toHaveBeenCalledWith(
          '1',
          'https://prod.mercadona.com/img/leche.jpg',
        ),
      );
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
    });

    it('shows an error alert if saving fails', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      vi.mocked(productsApi.updateProductImage).mockRejectedValue(new Error('Network error'));
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      fireEvent.change(screen.getByLabelText('URL de imagen del producto'), {
        target: { value: 'https://example.com/img.jpg' },
      });
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      await waitFor(() =>
        expect(screen.getByRole('alert')).toBeInTheDocument(),
      );
    });

    it('shows validation error when saving with empty URL', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.click(screen.getByRole('button', { name: 'Guardar imagen' }));
      expect(screen.getByRole('alert')).toHaveTextContent('Introduce una URL');
      expect(productsApi.updateProductImage).not.toHaveBeenCalled();
    });

    it('closes input on Escape key', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(mockProduct);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      await userEvent.click(screen.getByRole('button', { name: 'Cambiar imagen del producto' }));
      await userEvent.keyboard('{Escape}');
      expect(screen.queryByLabelText('URL de imagen del producto')).not.toBeInTheDocument();
    });
  });

  describe('DeletePriceRecord', () => {
    const productWithRecordIds: Product = {
      ...mockProduct,
      priceHistory: [
        { date: '2025-01-15T00:00:00Z', price: 0.79, store: 'Mercadona', recordId: 10 },
        { date: '2025-09-22T00:00:00Z', price: 0.89, store: 'Mercadona', recordId: 11 },
      ],
    };

    it('does not show delete column when token is not provided', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      render(<ProductDetail productId="1" onBack={vi.fn()} />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      expect(document.querySelector('.price-record-delete-btn')).not.toBeInTheDocument();
    });

    it('shows delete buttons when token is provided', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      render(<ProductDetail productId="1" onBack={vi.fn()} token="tok" />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      expect(document.querySelectorAll('.price-record-delete-btn')).toHaveLength(2);
    });

    it('shows inline confirm when trash button is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      render(<ProductDetail productId="1" onBack={vi.fn()} token="tok" />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      const [firstBtn] = document.querySelectorAll('.price-record-delete-btn');
      await userEvent.click(firstBtn);
      expect(screen.getByRole('button', { name: 'Eliminar' })).toBeInTheDocument();
      expect(screen.getByRole('button', { name: 'Cancelar' })).toBeInTheDocument();
    });

    it('hides confirm when Cancelar is clicked', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      render(<ProductDetail productId="1" onBack={vi.fn()} token="tok" />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      const [firstBtn] = document.querySelectorAll('.price-record-delete-btn');
      await userEvent.click(firstBtn);
      await userEvent.click(screen.getByRole('button', { name: 'Cancelar' }));
      expect(screen.queryByRole('button', { name: 'Eliminar' })).not.toBeInTheDocument();
    });

    it('calls deletePriceRecord and removes the row on confirm', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      vi.mocked(productsApi.deletePriceRecord).mockResolvedValue(undefined);
      render(<ProductDetail productId="1" onBack={vi.fn()} token="tok" />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      // The table is reversed; first visible row is recordId 11
      const [firstBtn] = document.querySelectorAll('.price-record-delete-btn');
      await userEvent.click(firstBtn);
      await userEvent.click(screen.getByRole('button', { name: 'Eliminar' }));
      await waitFor(() =>
        expect(productsApi.deletePriceRecord).toHaveBeenCalledWith('1', 11),
      );
      // After deletion only one Mercadona entry remains
      await waitFor(() =>
        expect(document.querySelectorAll('.price-record-delete-btn')).toHaveLength(1),
      );
    });

    it('shows delete error alert when deletePriceRecord fails', async () => {
      vi.mocked(productsApi.getProduct).mockResolvedValue(productWithRecordIds);
      vi.mocked(productsApi.deletePriceRecord).mockRejectedValue(new Error('Network error'));
      render(<ProductDetail productId="1" onBack={vi.fn()} token="tok" />);
      await waitFor(() => screen.getByText('LECHE ENTERA HACENDADO 1L'));
      const [firstBtn] = document.querySelectorAll('.price-record-delete-btn');
      await userEvent.click(firstBtn);
      await userEvent.click(screen.getByRole('button', { name: 'Eliminar' }));
      await waitFor(() =>
        expect(screen.getByRole('alert')).toHaveTextContent('No se pudo eliminar'),
      );
    });
  });
});
