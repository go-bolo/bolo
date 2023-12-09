# Bolo

Bolo is a powerful and high-performance web framework for Go, equipped with a built-in plugin system and an extensive range of modules to streamline web application development. Designed with performance and scalability in mind, Bolo empowers developers to build robust web applications effortlessly.

**Note: Bolo is currently under active development, and as such, it may undergo significant changes. Please exercise caution when using it in production environments.**

## Features

- **High performance:** Bolo is meticulously crafted to be fast and efficient, making it the ideal choice for building high-performance web programs, allowing you to build scalable applications.
- **Plugin system:** The framework use a flexible and adaptable plugin system, enabling developers to extend its functionality effortlessly. You can easily customize your application by integrating various plugins.
- **Modularity:** Bolo provides an assortment of modules to simplify common tasks like routing, authentication, database integration, and more. Developers have the flexibility to cherry-pick the modules that best suit their project's requirements and seamlessly integrate them.
- **Ease of use:** With a simple and intuitive API, Bolo allows developers, even those new to the Go language, to swiftly develop web applications without a steep learning curve.
- **Middleware Support:** Bolo comes with robust middleware support, allowing developers to enhance the request-response cycle with pre-processing and post-processing tasks. You can easily integrate middleware into your application's pipeline to add functionalities like logging, compression, and authentication.
- **WebSocket Support:** Bolo includes built-in WebSocket support, enabling real-time bidirectional communication between clients and the server. You can easily implement interactive features such as chat applications, live notifications, and collaborative tools using Bolo's WebSocket capabilities.

## Installation

To get started with Bolo, ensure you have Go installed and properly configured on your system. Once Go is set up, you can install the framework using the following command:

```bash
go get github.com/go-bolo/bolo
```

## Getting Started

Here's a simple example of how you create a simple Bolo application without plugins and MVC structure:

```golang
package main

import (
	"github.com/go-bolo/bolo"
)

func main() {
	app := bolo.New()
	app.Get("/", func(c *bolo.Context) {
		c.String(200, "Hello, Bolo!")
	})

	app.GetRouter().GET("/api", func(c *bolo.Context) {
		c.String(200, "Hello, Bolo!")
	})

	err = app.Bootstrap()
	if err != nil {
		panic(err)
	}

	err := app.StartHTTPServer()
	if err != nil {
		panic(err)
	}

}

```


For more detailed instructions and examples, please refer to the documentation.

## Core events

Bolo core event is powered by: https://github.com/gookit/event

- **configuration:** This event is triggered during the application's initialization phase when the configuration is being loaded. Developers can use this event to modify or extend the configuration before it's used by the application.
- **bindMiddlewares:** When this event is fired, Bolo is ready to bind middleware functions to the application's request-response cycle. Developers can register their custom middleware or perform additional setup for existing middleware.
- **bindRoutes:** This event is fired when the application is ready to bind routes to the router. Developers can use this event to register their application's routes, defining the URL endpoints and their corresponding handlers.
- **setResponseFormats:** When this event is triggered, developers can define or modify the supported response formats for the application.
- **bootstrap:** The 'bootstrap' event is the last event fired during the application's initialization process, indicating that the application is fully initialized and ready to start serving requests. Developers can perform any final setup or initialization tasks here.

## Testing and Benchmarking

Bolo provides built-in testing and benchmarking support, allowing you to ensure the reliability and performance of your web applications effortlessly.

## Contributing

We welcome and encourage contributions from the community. If you have any ideas, suggestions, or bug report, please don't hesitate to open an issue or submit a pull request on the GitHub repository.

## Tricks and Tips

- **Use Plugins Wisely:** Take advantage of Bolo's plugin system to enhance the functionality of your web application. Be cautious when selecting plugins and ensure they are well-maintained and compatible with the Bolo version you are using.
- **Leverage Modularity:** Bolo's modular design allows you to pick and choose only the components you need for your project. This can help keep your application lightweight and efficient.
- **Benchmark Your Code:** Use Bolo's built-in benchmarking support to identify performance bottlenecks in your code. Regularly benchmark your application to ensure it meets your performance requirements.
- **Error Handling Best Practices:** Implement robust error handling in your application to provide meaningful feedback to users and make troubleshooting easier.
- **Security Considerations:** When deploying Bolo in production environments, pay special attention to security measures, such as input validation, secure authentication, and protection against common web vulnerabilities.
- **Cautious Shutdown:** Implement a graceful shutdown mechanism for your Bolo application to handle server shutdowns smoothly and avoid potential data loss or corruption.
- **Join the Community:** Engage with the Bolo community to seek help, share experiences, and contribute to the project's development.
- **Stay Updated:** Keep an eye on Bolo's GitHub repository for updates, bug fixes, and new features. Regularly update your Bolo installation to benefit from the latest improvements.

## Core events:

### Request lifecycle

- `set-default-request-context-values`
  - Set default values on echo context:


## License

Bolo is released under the MIT License.

Bolo is a game-changer for Go developers, providing a combination of speed, flexibility, and simplicity. Whether you're building a small web application or a large-scale project, Bolo's powerful features and modular approach will help you create top-notch web experiences with ease. Join the Bolo community today and elevate your web development journey to new heights!
