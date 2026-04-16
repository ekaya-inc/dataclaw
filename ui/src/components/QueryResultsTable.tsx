import { Card, CardContent, CardHeader, CardTitle } from './ui/Card';

export function QueryResultsTable({
  columns,
  rows,
  rowCount,
}: {
  columns: Array<{ name: string; type: string }>;
  rows: Record<string, unknown>[];
  rowCount: number;
}): JSX.Element {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-lg">Query results</CardTitle>
        <p className="text-sm text-text-secondary">Returned {rowCount} rows.</p>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-sm text-text-secondary">No rows returned.</p>
        ) : (
          <div className="overflow-x-auto rounded-xl border border-border-light">
            <table className="min-w-full divide-y divide-border-light text-sm">
              <thead className="bg-surface-secondary">
                <tr>
                  {columns.map((column) => (
                    <th key={column.name} className="px-4 py-3 text-left font-semibold text-text-primary">
                      <div>{column.name}</div>
                      <div className="text-xs font-normal text-text-tertiary">{column.type}</div>
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody className="divide-y divide-border-light bg-surface-primary">
                {rows.map((row, index) => (
                  <tr key={index}>
                    {columns.map((column) => (
                      <td key={`${index}-${column.name}`} className="px-4 py-3 align-top text-text-primary">
                        {String(row[column.name] ?? '')}
                      </td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
