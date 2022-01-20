use std::net::{Ipv4Addr, TcpListener, TcpStream};

const LISTENING_PORT: u16 = 1234;

fn main() -> std::io::Result<()> {
    let local_addr = &if_addrs::get_if_addrs()
        .unwrap()
        .into_iter()
        .find(|iface| iface.name == "eth1")
        .unwrap()
        .addr
        .ip();

    println!("Data network local_addr: {:?}", local_addr);

    match local_addr {
        std::net::IpAddr::V4(addr) if addr.octets()[3] == 2 => {
            println!("Test instance, listening for incoming connections.");

            let listener = TcpListener::bind((*addr, LISTENING_PORT))?;

            for _stream in listener.incoming() {
                println!("Established inbound TCP connection.");
            }
        }
        std::net::IpAddr::V4(addr) if addr.octets()[3] == 3 => {
            println!("Test instance, connecting to listening instance.");

            // Wait for listening instance to bind to port.
            std::thread::sleep(std::time::Duration::from_secs(2));

            let remote_addr: Ipv4Addr = {
                let mut octets = addr.octets();
                octets[3] = 2;
                octets.into()
            };
            let _stream = TcpStream::connect((remote_addr, LISTENING_PORT)).unwrap();
            println!("Established outbound TCP connection.");
        }
        addr => {
            panic!("Unexpected local IP address {:?}", addr);
        }
    }

    Ok(())
}
