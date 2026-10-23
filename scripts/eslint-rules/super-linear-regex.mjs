import scslre from "scslre";

// The detector SonarQube runs for javascript:S8786. Without it a regex with a
// quadratic worst case only surfaces in the SonarCloud report days later.
export const superLinearRegex = {
    meta: {
        type: "problem",
        docs: {
            description: "Disallow regular expressions whose worst-case runtime is super-linear",
        },
        schema: [],
        messages: {
            superLinear: "This regular expression backtracks super-linearly ({{type}}). Make every quantified group unambiguous so the match runs in linear time.",
        },
    },
    create(context) {
        return {
            Literal(node) {
                if (!node.regex) {
                    return;
                }

                let analysis;
                try {
                    analysis = scslre.analyse(new RegExp(node.regex.pattern, node.regex.flags));
                } catch {
                    return;
                }

                if (analysis.reports.length === 0) {
                    return;
                }

                const types = [...new Set(analysis.reports.map((report) => report.type))].join(", ");
                context.report({ node, messageId: "superLinear", data: { type: types } });
            },
        };
    },
};
