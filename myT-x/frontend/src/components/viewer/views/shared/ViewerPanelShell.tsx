import type {ReactNode} from "react";
import {ViewerHeader} from "./ViewerHeader";

/** Common props shared by both message-mode and children-mode. */
interface ViewerPanelShellBase {
    readonly className: string;
    readonly title: string;
    readonly onClose: () => void;
    readonly onRefresh?: () => void;
    readonly refreshTitle?: string;
    readonly headerChildren?: ReactNode;
}

/**
 * Discriminated union: exactly one of `message` or `children` may be provided.
 * Passing both is a compile-time error.
 */
type ViewerPanelShellProps =
    | (ViewerPanelShellBase & {
    /** Non-empty status/error message. When provided, children must be omitted. */
    readonly message: string;
    readonly children?: never;
})
    | (ViewerPanelShellBase & { readonly message?: never; readonly children?: ReactNode });

/**
 * Shared viewer shell for the common "header + message/body" layout.
 *
 * Use `message` for status/error screens (no children allowed).
 * Omit `message` and pass `children` for the main content body.
 */
export function ViewerPanelShell(props: ViewerPanelShellProps) {
    const {className, title, onClose, onRefresh, refreshTitle, headerChildren, message} = props;
    // Truthy check: message is always a non-empty string when the message variant
    // is used (callers pass user-facing error/status text). The `?? never` variant
    // makes message undefined when children are passed, so this correctly picks
    // the children branch. Empty string "" is falsy and treated as no-message.
    const body = message
        ? <div className="viewer-message">{message}</div>
        : props.children ?? null;
    return (
        <div className={className}>
            <ViewerHeader
                title={title}
                onClose={onClose}
                onRefresh={onRefresh}
                refreshTitle={refreshTitle}
            >
                {headerChildren}
            </ViewerHeader>
            {body}
        </div>
    );
}
