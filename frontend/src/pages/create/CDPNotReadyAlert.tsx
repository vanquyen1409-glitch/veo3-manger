import { TriangleAlert, ArrowRight, RefreshCw } from 'lucide-react';
import { Button } from '../../components/ui/Button';
import { Card } from '../../components/ui/Card';

interface CDPNotReadyAlertProps {
  message: string;
  onRefresh: () => void;
  onOpenSettings: () => void;
}

// Warning card shown on the Create page when the CDP browser isn't ready,
// nudging the user to fix it (refresh status, or jump to Settings).
export function CDPNotReadyAlert({ message, onRefresh, onOpenSettings }: CDPNotReadyAlertProps) {
  return (
    <Card className="border-amber-200 bg-amber-50">
      <div className="flex items-start gap-3">
        <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-amber-600" />
        <div className="flex-1">
          <h3 className="text-sm font-semibold text-amber-900">Trình duyệt CDP chưa sẵn sàng</h3>
          <p className="mt-0.5 text-sm text-amber-800">{message}</p>
          <div className="mt-3 flex gap-2">
            <Button
              variant="secondary"
              size="sm"
              leftIcon={<RefreshCw className="h-4 w-4" />}
              onClick={onRefresh}
            >
              Làm mới trạng thái
            </Button>
            <Button
              variant="primary"
              size="sm"
              rightIcon={<ArrowRight className="h-4 w-4" />}
              onClick={onOpenSettings}
            >
              Mở Cài Đặt
            </Button>
          </div>
        </div>
      </div>
    </Card>
  );
}
