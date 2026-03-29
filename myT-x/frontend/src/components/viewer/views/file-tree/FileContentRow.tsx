import {memo} from "react";
import type {ListChildComponentProps} from "react-window";
import type {ThemedToken} from "shiki/core";
import {sanitizeCssColor} from "../../../../utils/cssUtils";

export interface FileContentRowData {
    readonly lines: readonly string[];
    readonly tokens: readonly (readonly ThemedToken[])[] | null;
}

export const FileContentRow = memo(function FileContentRow({index, style, data}: ListChildComponentProps<FileContentRowData>) {
    const line = data.lines[index] ?? "";
    const lineTokens = data.tokens?.[index];
    return (
        <div style={style} className="file-content-line" data-line-index={index}>
            <span className="file-content-line-number" aria-hidden="true">{index + 1}</span>
            <span className="file-content-line-text">
                {lineTokens ? (
                    lineTokens.map((token, tokenIndex) => (
                        <span key={`token-${index}-${tokenIndex}`} style={{color: sanitizeCssColor(token.color)}}>
                            {token.content}
                        </span>
                    ))
                ) : (
                    line
                )}
            </span>
        </div>
    );
});
