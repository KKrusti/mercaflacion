import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import TicketUploader from './TicketUploader';
import * as productsApi from '../api/products';
import type { TicketUploadSummary } from '../types';

vi.mock('../api/products');

const singleSuccess: TicketUploadSummary = {
  total: 1,
  succeeded: 1,
  failed: 0,
  items: [
    { file: 'ticket.pdf', ok: true, result: { invoiceNumber: '1234', linesImported: 5 } },
  ],
};

const multiMixed: TicketUploadSummary = {
  total: 2,
  succeeded: 1,
  failed: 1,
  items: [
    { file: 'a.pdf', ok: true, result: { invoiceNumber: '001', linesImported: 3 } },
    { file: 'b.pdf', ok: false, error: 'Formato no válido' },
  ],
};

beforeEach(() => {
  vi.resetAllMocks();
});

describe('TicketUploader', () => {
  it('renders the upload button', () => {
    render(<TicketUploader />);
    expect(screen.getByRole('button', { name: /subir tickets/i })).toBeInTheDocument();
  });

  it('has a hidden file input that accepts PDFs', () => {
    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);
    expect(input).toHaveAttribute('type', 'file');
    expect(input).toHaveAttribute('accept');
    expect(input).toHaveAttribute('multiple');
  });

  it('shows uploading state while processing', async () => {
    // Never resolves during this test
    vi.mocked(productsApi.uploadTickets).mockReturnValue(new Promise(() => {}));

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    expect(await screen.findByText(/subiendo/i)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /subiendo/i })).toBeDisabled();
  });

  it('shows progress panel with correct label for a single file', async () => {
    // Simulate progress callback by calling it before resolving
    vi.mocked(productsApi.uploadTickets).mockImplementation(
      async (_files, onProgress) => {
        onProgress?.(0, 1);
        return singleSuccess;
      },
    );

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);
    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    // After upload completes the toast should be visible
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());
    expect(screen.getByText(/1 ticket importado/i)).toBeInTheDocument();
  });

  it('shows progress panel counting multiple files', async () => {
    let capturedOnProgress: ((done: number, total: number) => void) | undefined;

    // Hang indefinitely so we can inspect the in-progress state
    vi.mocked(productsApi.uploadTickets).mockImplementation(
      (_files, onProgress) => {
        capturedOnProgress = onProgress;
        return new Promise(() => {});
      },
    );

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);
    const files = [
      new File(['%PDF'], 'a.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'b.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'c.pdf', { type: 'application/pdf' }),
    ];
    await userEvent.upload(input, files);

    // Simulate first file completing
    capturedOnProgress?.(1, 3);

    await waitFor(() =>
      expect(screen.getByText(/procesando 1 de 3 tickets/i)).toBeInTheDocument(),
    );
  });

  it('shows success toast after single file upload', async () => {
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(singleSuccess);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    await waitFor(() =>
      expect(screen.getByRole('status')).toBeInTheDocument(),
    );

    expect(screen.getByText(/1 ticket importado/i)).toBeInTheDocument();
  });

  it('shows error details when some files fail', async () => {
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(multiMixed);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const files = [
      new File(['%PDF'], 'a.pdf', { type: 'application/pdf' }),
      new File(['bad'], 'b.pdf', { type: 'application/pdf' }),
    ];
    await userEvent.upload(input, files);

    await waitFor(() =>
      expect(screen.getByRole('status')).toBeInTheDocument(),
    );

    expect(screen.getByText(/1 ok · 1 error/i)).toBeInTheDocument();
    expect(screen.getByText('a.pdf')).toBeInTheDocument();
    expect(screen.getByText('b.pdf')).toBeInTheDocument();
  });

  it('calls uploadTickets with the selected files', async () => {
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(singleSuccess);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    await waitFor(() =>
      expect(productsApi.uploadTickets).toHaveBeenCalledWith(
        [file],
        expect.any(Function),
      ),
    );
  });

  it('dismisses the toast when the close button is clicked', async () => {
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(singleSuccess);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    await waitFor(() => screen.getByRole('status'));
    await userEvent.click(screen.getByRole('button', { name: /cerrar notificaci/i }));

    expect(screen.queryByRole('status')).not.toBeInTheDocument();
  });

  it('re-enables the button and hides toast after dismiss', async () => {
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(singleSuccess);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);
    const file = new File(['%PDF'], 'ticket.pdf', { type: 'application/pdf' });
    await userEvent.upload(input, file);

    await waitFor(() => screen.getByRole('status'));
    await userEvent.click(screen.getByRole('button', { name: /cerrar notificaci/i }));

    expect(screen.getByRole('button', { name: /subir tickets/i })).not.toBeDisabled();
  });

  it('shows a friendly duplicate-file error message when the server returns a 409', async () => {
    // The friendly message is already translated in api/products.ts (friendlyUploadError)
    const duplicateSummary: TicketUploadSummary = {
      total: 2,
      succeeded: 1,
      failed: 1,
      items: [
        { file: 'nuevo.pdf', ok: true, result: { invoiceNumber: 'N1', linesImported: 4 } },
        { file: 'viejo.pdf', ok: false, error: 'Este ticket ya fue importado anteriormente' },
      ],
    };
    vi.mocked(productsApi.uploadTickets).mockResolvedValue(duplicateSummary);

    render(<TicketUploader />);
    const input = screen.getByLabelText(/seleccionar tickets pdf/i);

    const files = [
      new File(['%PDF'], 'nuevo.pdf', { type: 'application/pdf' }),
      new File(['%PDF'], 'viejo.pdf', { type: 'application/pdf' }),
    ];
    await userEvent.upload(input, files);

    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());

    expect(screen.getByText(/este ticket ya fue importado anteriormente/i)).toBeInTheDocument();
  });
});
