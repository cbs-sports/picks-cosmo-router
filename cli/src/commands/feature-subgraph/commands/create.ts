import { existsSync } from 'node:fs';
import { readFile } from 'node:fs/promises';
import { EnumStatusCode } from '@wundergraph/cosmo-connect/dist/common/common_pb';
import { parseGraphQLSubscriptionProtocol, parseGraphQLWebsocketSubprotocol } from '@wundergraph/cosmo-shared';
import { Command, program } from 'commander';
import ora from 'ora';
import { resolve } from 'pathe';
import pc from 'picocolors';
import { getBaseHeaders } from '../../../core/config.js';
import { BaseCommandOptions } from '../../../core/types/types.js';
import { validateSubscriptionProtocols } from '../../../utils.js';
import { websocketSubprotocolDescription } from '../../../constants.js';

export default (opts: BaseCommandOptions) => {
  const command = new Command('create');
  command.description('Creates a feature subgraph on the control plane.');
  command.argument('<name>', 'The name of the feature subgraph to create.');
  command.option('-n, --namespace [string]', 'The namespace of the feature subgraph.');
  command.requiredOption(
    '-r, --routing-url <url>',
    'The routing url of your feature subgraph. This is the url at which the feature subgraph will be accessible.',
  );
  command.option(
    '--subscription-url [url]',
    'The url used for subscriptions. If empty, it defaults to same url used for routing.',
  );
  command.option(
    '--subscription-protocol <protocol>',
    'The protocol to use when subscribing to the feature subgraph. The supported protocols are ws, sse, and sse_post.',
  );
  command.option('--websocket-subprotocol <protocol>', websocketSubprotocolDescription);
  command.option('--readme <path-to-readme>', 'The markdown file which describes the feature subgraph.');
  command.requiredOption('--subgraph <subgraph>', 'The subgraph name for which the feature subgraph is to be created');
  command.action(async (name, options) => {
    let readmeFile;
    if (options.readme) {
      readmeFile = resolve(options.readme);
      if (!existsSync(readmeFile)) {
        program.error(
          pc.red(
            pc.bold(`The readme file '${pc.bold(readmeFile)}' does not exist. Please check the path and try again.`),
          ),
        );
      }
    }

    validateSubscriptionProtocols({
      subscriptionProtocol: options.subscriptionProtocol,
      websocketSubprotocol: options.websocketSubprotocol,
    });

    const spinner = ora(`The feature subgraph "${name}" is being created...`).start();
    const resp = await opts.client.platform.createFederatedSubgraph(
      {
        name,
        namespace: options.namespace,
        labels: [],
        routingUrl: options.routingUrl,
        // If the argument is provided but the URL is not, clear it
        subscriptionUrl: options.subscriptionUrl === true ? '' : options.subscriptionUrl,
        subscriptionProtocol: options.subscriptionProtocol
          ? parseGraphQLSubscriptionProtocol(options.subscriptionProtocol)
          : undefined,
        websocketSubprotocol: options.websocketSubprotocol
          ? parseGraphQLWebsocketSubprotocol(options.websocketSubprotocol)
          : undefined,
        readme: readmeFile ? await readFile(readmeFile, 'utf8') : undefined,
        isFeatureSubgraph: true,
        baseSubgraphName: options.subgraph,
      },
      {
        headers: getBaseHeaders(),
      },
    );

    if (resp.response?.code === EnumStatusCode.OK) {
      spinner.succeed(`The feature subgraph "${name}" was created successfully.`);
    } else {
      spinner.fail(`Failed to create the feature subgraph "${name}".`);
      if (resp.response?.details) {
        console.log(pc.red(pc.bold(resp.response?.details)));
      }
      process.exitCode = 1;
      // eslint-disable-next-line no-useless-return
      return;
    }
  });

  return command;
};
