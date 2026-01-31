import json
from typing import Any, Iterable, Optional


class WrappedError(Exception):
    def __init__(
        self,
        code: str = "",
        message: str = "",
        file: str = "",
        line: int = 0,
        cause: Optional[BaseException] = None,
    ):
        super().__init__(message)
        self.code = code or ""
        self.message = message or ""
        self.file = file or ""
        self.line = int(line or 0)
        self.cause = cause

    def __str__(self) -> str:
        return "\n".join(self._format_lines())

    def _format_lines(self) -> Iterable[str]:
        current: Optional[BaseException] = self
        while current is not None:
            if isinstance(current, WrappedError):
                location = ""
                if current.file:
                    location = f"{current.file}"
                    if current.line:
                        location = f"{location}:{current.line}"
                parts = [p for p in [current.code, location, current.message] if p]
                yield " - ".join(parts) if parts else "WrappedError"
                current = current.cause
            else:
                yield str(current)
                current = getattr(current, "__cause__", None)

    def has_code(self, *codes: str) -> bool:
        wanted = {c for c in codes if c}
        if not wanted:
            return False
        current: Optional[BaseException] = self
        while current is not None:
            if isinstance(current, WrappedError):
                if current.code in wanted:
                    return True
                current = current.cause
            else:
                return False
        return False

    def __contains__(self, code: str) -> bool:
        return self.has_code(code)

    @classmethod
    def from_payload(cls, payload: Any) -> Optional["WrappedError"]:
        if payload is None:
            return None
        if isinstance(payload, WrappedError):
            return payload
        if isinstance(payload, BaseException) and not isinstance(payload, dict):
            return cls(message=str(payload))
        if isinstance(payload, str):
            try:
                payload = json.loads(payload)
            except json.JSONDecodeError:
                return None
        if isinstance(payload, dict):
            code = payload.get("code", "") or ""
            message = payload.get("msg", "") or payload.get("message", "") or ""
            file = payload.get("file", "") or ""
            line = payload.get("line", 0) or 0
            cause_payload = payload.get("cause")
            cause = cls.from_payload(cause_payload) if cause_payload is not None else None
            return cls(code=code, message=message, file=file, line=line, cause=cause)
        return None


def has_code(err: BaseException, *codes: str) -> bool:
    if isinstance(err, WrappedError):
        return err.has_code(*codes)
    wrapped = WrappedError.from_payload(err)
    return wrapped.has_code(*codes) if wrapped else False
